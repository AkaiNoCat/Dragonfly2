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

package dfget

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"d7y.io/dragonfly/v2/client/config"
	"d7y.io/dragonfly/v2/internal/dfheaders"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/basic"
	"d7y.io/dragonfly/v2/pkg/rpc/base"
	"d7y.io/dragonfly/v2/pkg/rpc/dfdaemon"
	daemonclient "d7y.io/dragonfly/v2/pkg/rpc/dfdaemon/client"
	"d7y.io/dragonfly/v2/pkg/source"
	"d7y.io/dragonfly/v2/pkg/util/digestutils"
	"d7y.io/dragonfly/v2/pkg/util/stringutils"
	"github.com/pkg/errors"
	"github.com/schollz/progressbar/v3"
)

func Download(cfg *config.DfgetConfig, client daemonclient.DaemonClient) error {
	var (
		ctx       = context.Background()
		cancel    context.CancelFunc
		wLog      = logger.With("url", cfg.URL)
		downError error
	)

	wLog.Info("init success and start to download")
	fmt.Println("init success and start to download")

	if cfg.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}

	go func() {
		defer cancel()
		downError = download(ctx, client, cfg, wLog)
	}()

	<-ctx.Done()

	if ctx.Err() == context.DeadlineExceeded {
		return errors.Errorf("download timeout(%s)", cfg.Timeout)
	}
	return downError
}

func download(ctx context.Context, client daemonclient.DaemonClient, cfg *config.DfgetConfig, wLog *logger.SugaredLoggerOnWith) error {
	hdr := parseHeader(cfg.Header)

	if client == nil {
		return downloadFromSource(ctx, cfg, hdr)
	}

	var (
		start     = time.Now()
		stream    *daemonclient.DownResultStream
		result    *dfdaemon.DownResult
		pb        *progressbar.ProgressBar
		request   = newDownRequest(cfg, hdr)
		downError error
	)

	if stream, downError = client.Download(ctx, request); downError == nil {
		if cfg.ShowProgress {
			pb = newProgressBar(-1)
		}

		for {
			if result, downError = stream.Recv(); downError != nil {
				break
			}

			if result.CompletedLength > 0 && pb != nil {
				_ = pb.Set64(int64(result.CompletedLength))
			}

			// success
			if result.Done {
				if pb != nil {
					pb.Describe("Downloaded")
					_ = pb.Close()
				}

				wLog.Infof("download from daemon success, length: %d bytes cost: %d ms", result.CompletedLength, time.Now().Sub(start).Milliseconds())
				fmt.Printf("finish total length %d bytes\n", result.CompletedLength)

				break
			}
		}
	}

	if downError != nil {
		wLog.Warnf("daemon downloads file error: %v", downError)
		fmt.Printf("daemon downloads file error: %v\n", downError)
		downError = downloadFromSource(ctx, cfg, hdr)
	}

	return downError
}

func downloadFromSource(ctx context.Context, cfg *config.DfgetConfig, hdr map[string]string) error {
	if cfg.DisableBackSource {
		return errors.New("try to download from source but back source is disabled")
	}

	var (
		wLog     = logger.With("url", cfg.URL)
		start    = time.Now()
		target   *os.File
		response io.ReadCloser
		err      error
		written  int64
	)

	wLog.Info("try to download from source and ignore rate limit")
	fmt.Println("try to download from source and ignore rate limit")

	if target, err = ioutil.TempFile(filepath.Dir(cfg.Output), ".df_"); err != nil {
		return err
	}
	defer os.Remove(target.Name())
	defer target.Close()

	downloadRequest, err := source.NewRequestWithContext(ctx, cfg.URL, hdr)
	if err != nil {
		return err
	}
	if response, err = source.Download(downloadRequest); err != nil {
		return err
	}
	defer response.Close()

	if written, err = io.Copy(target, response); err != nil {
		return err
	}

	if !stringutils.IsBlank(cfg.Digest) {
		parsedHash := digestutils.Parse(cfg.Digest)
		realHash := digestutils.HashFile(target.Name(), digestutils.Algorithms[parsedHash[0]])

		if realHash != "" && realHash != parsedHash[1] {
			return errors.Errorf("%s digest is not matched: real[%s] expected[%s]", parsedHash[0], realHash, parsedHash[1])
		}
	}

	// change file owner
	if err = os.Chown(target.Name(), basic.UserID, basic.UserGroup); err != nil {
		return errors.Wrapf(err, "change file owner to uid[%d] gid[%d]", basic.UserID, basic.UserGroup)
	}

	if err = os.Rename(target.Name(), cfg.Output); err != nil {
		return err
	}

	wLog.Infof("download from source success, length: %d bytes cost: %d ms", written, time.Now().Sub(start).Milliseconds())
	fmt.Printf("finish total length %d bytes\n", written)

	return nil
}

func parseHeader(s []string) map[string]string {
	hdr := make(map[string]string)
	var key, value string
	for _, h := range s {
		idx := strings.Index(h, ":")
		if idx > 0 {
			key = strings.TrimSpace(h[:idx])
			value = strings.TrimSpace(h[idx+1:])
			hdr[key] = value
		}
	}

	return hdr
}

func newDownRequest(cfg *config.DfgetConfig, hdr map[string]string) *dfdaemon.DownRequest {
	return &dfdaemon.DownRequest{
		Url:               cfg.URL,
		Output:            cfg.Output,
		Timeout:           int64(cfg.Timeout),
		Limit:             float64(cfg.RateLimit),
		DisableBackSource: cfg.DisableBackSource,
		UrlMeta: &base.UrlMeta{
			Digest: cfg.Digest,
			Tag:    cfg.Tag,
			Range:  hdr[dfheaders.Range],
			Filter: cfg.Filter,
			Header: hdr,
		},
		Pattern:    cfg.Pattern,
		Callsystem: cfg.CallSystem,
		Uid:        int64(basic.UserID),
		Gid:        int64(basic.UserGroup),
	}
}

func newProgressBar(max int64) *progressbar.ProgressBar {
	return progressbar.NewOptions64(max,
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowIts(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionUseANSICodes(true),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetDescription("[cyan]Downloading...[reset]"),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}))
}
