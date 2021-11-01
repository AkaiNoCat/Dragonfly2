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
//go:generate mockgen -destination ./mock/mock_progress_mgr.go -package mock d7y.io/dragonfly/v2/cdn/supervisor SeedProgressManager

package supervisor

import (
	"context"

	"d7y.io/dragonfly/v2/cdn/types"
)

// SeedProgressManager as an interface defines all operations about seed progress
type SeedProgressManager interface {

	// InitSeedProgress init task seed progress
	InitSeedProgress(ctx context.Context, taskID string)

	// WatchSeedProgress watch task seed progress
	WatchSeedProgress(ctx context.Context, task *types.SeedTask) (<-chan *types.SeedPiece, error)

	// PublishPiece publish piece seed
	PublishPiece(ctx context.Context, taskID string, piece *types.SeedPiece) error

	// PublishTask publish task seed
	PublishTask(ctx context.Context, taskID string, task *types.SeedTask) error

	// GetPieces get pieces by taskID
	GetPieces(ctx context.Context, taskID string) (records []*types.SeedPiece, ok bool)

	// Clear meta info of task
	Clear(taskID string)
}
