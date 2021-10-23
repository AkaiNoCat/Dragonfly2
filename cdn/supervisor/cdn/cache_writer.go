/*
 *     Copyright 2020 The Dragonfly Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cdn

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"d7y.io/dragonfly/v2/cdn/config"
	"d7y.io/dragonfly/v2/cdn/supervisor/cdn/storage"
	"d7y.io/dragonfly/v2/cdn/types"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/ratelimiter/limitreader"
	"d7y.io/dragonfly/v2/pkg/util/digestutils"
	"d7y.io/dragonfly/v2/pkg/util/rangeutils"
	"go.opentelemetry.io/otel/trace"
)

type piece struct {
	taskID       string
	pieceNum     int32
	pieceSize    int32
	pieceContent *bytes.Buffer
}

type downloadMetadata struct {
	backSourceLength     int64 // back to source download file length
	realCdnFileLength    int64 // the actual length of the stored file
	realSourceFileLength int64 // actually read the length of the source
	totalPieceCount      int32 // piece total count
	pieceMd5Sign         string
	sourceRealDigest     string
}

type cacheWriter struct {
	cdnReporter *reporter
	cacheStore  storage.Manager
}

func newCacheWriter(cdnReporter *reporter, cacheStore storage.Manager) *cacheWriter {
	return &cacheWriter{
		cdnReporter: cdnReporter,
		cacheStore:  cacheStore,
	}
}

// startWriter writes the stream data from the reader to the underlying storage.
func (cw *cacheWriter) startWriter(ctx context.Context, reader *limitreader.LimitReader, task *types.SeedTask, breakPoint int64) (*downloadMetadata, error) {
	var writeSpan trace.Span
	ctx, writeSpan = tracer.Start(ctx, config.SpanWriteData)
	defer writeSpan.End()
	// currentSourceFileLength is used to calculate the source file Length dynamically
	currentSourceFileLength := breakPoint
	// the pieceNum currently have been processed
	curPieceNum := len(detectResult.pieceMetaRecords)
	routineCount := calculateRoutineCount(task.SourceFileLength-currentSourceFileLength, task.PieceSize)
	writeSpan.SetAttributes(config.AttributeWriteGoroutineCount.Int(routineCount))
	// start writer pool
	backSourceLength, totalPieceCount, err := cw.doWrite(ctx, reader, task, routineCount, curPieceNum)
	if err != nil {
		return &downloadMetadata{backSourceLength: backSourceLength}, fmt.Errorf("write data: %v", err)
	}
	storageInfo, err := cw.cacheStore.statDownloadFile(task.ID)
	if err != nil {
		return &downloadMetadata{backSourceLength: backSourceLength}, fmt.Errorf("stat cdn download file: %v", err)
	}
	storageInfoBytes, _ := json.Marshal(storageInfo)
	writeSpan.SetAttributes(config.AttributeDownloadFileInfo.String(string(storageInfoBytes)))
	// TODO Try getting it from the ProgressManager first
	pieceMd5Sign, _, err := cw.cacheDataManager.getPieceMd5Sign(task.ID)
	if err != nil {
		return &downloadMetadata{backSourceLength: backSourceLength}, fmt.Errorf("get piece md5 sign: %v", err)
	}
	return &downloadMetadata{
		backSourceLength:     backSourceLength,
		realCdnFileLength:    storageInfo.Size,
		realSourceFileLength: currentSourceFileLength + backSourceLength,
		totalPieceCount:      int32(totalPieceCount),
		pieceMd5Sign:         pieceMd5Sign,
		sourceRealDigest:     reader.Digest(),
	}, nil
}

// doWrite do actual write data to storage
func (cw *cacheWriter) doWrite(ctx context.Context, reader io.Reader, task *types.SeedTask, routineCount int, curPieceNum int) (n int64, totalPiece int,
	err error) {
	var bufPool = &sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
	var backSourceLength int64
	buf := make([]byte, 256*1024)
	jobCh := make(chan *piece)
	var wg = &sync.WaitGroup{}
	cw.writerPool(ctx, wg, routineCount, jobCh, bufPool)
	for {
		var bb = bufPool.Get().(*bytes.Buffer)
		bb.Reset()
		limitReader := io.LimitReader(reader, int64(task.PieceSize))
		n, err = io.CopyBuffer(bb, limitReader, buf)
		if err != nil {
			close(jobCh)
			return backSourceLength, 0, fmt.Errorf("read source taskID %s pieceNum %d piece: %v", task.ID, curPieceNum, err)
		}
		if n == 0 {
			break
		}
		backSourceLength += n

		jobCh <- &piece{
			taskID:       task.ID,
			pieceNum:     int32(curPieceNum),
			pieceSize:    task.PieceSize,
			pieceContent: bb,
		}
		curPieceNum++
		if n < int64(task.PieceSize) {
			break
		}
	}
	close(jobCh)
	wg.Wait()
	return backSourceLength, curPieceNum, nil
}

func (cw *cacheWriter) writerPool(ctx context.Context, wg *sync.WaitGroup, routineCount int, pieceCh chan *piece, bufPool *sync.Pool) {
	wg.Add(routineCount)
	for i := 0; i < routineCount; i++ {
		go func() {
			defer wg.Done()
			for p := range pieceCh {
				// TODO Subsequent compression and other features are implemented through waitToWriteContent and pieceStyle
				waitToWriteContent := p.pieceContent
				originPieceLen := waitToWriteContent.Len() // the length of the original data that has not been processed
				pieceLen := originPieceLen                 // the real length written to the storage medium after processing
				pieceStyle := types.PlainUnspecified
				pieceMd5 := md5.New()
				err := cw.cacheDataManager.writeDownloadFile(
					p.taskID, int64(p.pieceNum)*int64(p.pieceSize), int64(waitToWriteContent.Len()),
					io.TeeReader(io.LimitReader(p.pieceContent, int64(waitToWriteContent.Len())), pieceMd5))
				// Recycle Buffer
				bufPool.Put(waitToWriteContent)
				if err != nil {
					logger.Errorf("write taskID %s pieceNum %d file: %v", p.taskID, p.pieceNum, err)
					continue
				}
				start := uint64(p.pieceNum) * uint64(p.pieceSize)
				end := start + uint64(pieceLen) - 1
				pieceRecord := &storage.PieceMetaRecord{
					PieceNum: p.pieceNum,
					PieceLen: int32(pieceLen),
					Md5:      digestutils.ToHashString(pieceMd5),
					Range: &rangeutils.Range{
						StartIndex: start,
						EndIndex:   end,
					},
					OriginRange: &rangeutils.Range{
						StartIndex: start,
						EndIndex:   end,
					},
					PieceStyle: pieceStyle,
				}
				// write piece meta to storage
				if err = cw.cacheDataManager.appendPieceMetadata(p.taskID, pieceRecord); err != nil {
					logger.Errorf("write piece meta file: %v", err)
					continue
				}

				if cw.cdnReporter != nil {
					if err = cw.cdnReporter.reportPieceMetaRecord(ctx, p.taskID, pieceRecord, DownloaderReport); err != nil {
						// NOTE: should we do this job again?
						logger.Errorf("report piece status, pieceNum %d pieceMetaRecord %s: %v", p.pieceNum, pieceRecord, err)
					}
				}
			}
		}()
	}
}

/*
	helper functions
*/

// calculateRoutineCount max goroutine count is CDNWriterRoutineLimit
func calculateRoutineCount(remainingFileLength int64, pieceSize int32) int {
	routineSize := config.CDNWriterRoutineLimit
	if remainingFileLength < 0 || pieceSize <= 0 {
		return routineSize
	}

	if remainingFileLength == 0 {
		return 1
	}

	tmpSize := (int)((remainingFileLength + int64(pieceSize-1)) / int64(pieceSize))
	if tmpSize == 0 {
		tmpSize = 1
	}
	if tmpSize < routineSize {
		routineSize = tmpSize
	}
	return routineSize
}
