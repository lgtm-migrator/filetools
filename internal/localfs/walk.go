// Copyright 2022 Jeremy Edwards
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package localfs

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
)

type ShardedWalkFn interface {
	NewWalkShard() func(string, os.FileInfo, error) error
}

func filesOnly(f func(string, os.FileInfo, error) error) func(string, os.FileInfo, error) error {
	return func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return nil
		}
		if info.Mode()&os.ModeType == 0 {
			f(path, info, err)
		}
		return nil
	}
}

func ConcurrentWalk(paths []string, walkFn ShardedWalkFn) error {
	paths, err := DirList(paths)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return nil
	}
	for _, path := range paths {
		if !DirExists(path) {
			return errors.Errorf("%s is not a directory", path)
		}
	}
	chErr := make(chan error, 1)
	defer func() {
		close(chErr)
	}()

	var wg sync.WaitGroup
	for _, path := range paths {
		path := path
		wg.Add(1)
		go func() {
			defer wg.Done()
			f := filesOnly(walkFn.NewWalkShard())
			walkErr := filepath.Walk(path, f)
			if walkErr != nil {
				select {
				case chErr <- walkErr:
				default:
					fmt.Printf("channel is full printing error, %s\n", walkErr)
				}
			}
		}()
	}
	wg.Wait()

	select {
	case err := <-chErr:
		return err
	default:
		return nil
	}
}

func Walk(paths []string, f func(string, os.FileInfo, error) error) error {
	paths, err := DirList(paths)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return nil
	}
	chErr := make(chan error, 1)
	defer func() {
		close(chErr)
	}()

	filesOnlyF := filesOnly(f)
	var wg sync.WaitGroup
	for _, path := range paths {
		path := path
		wg.Add(1)
		go func() {
			defer wg.Done()
			walkErr := filepath.Walk(path, filesOnlyF)
			if walkErr != nil {
				select {
				case chErr <- walkErr:
				default:
					fmt.Printf("channel is full printing error, %s\n", walkErr)
				}
			}
		}()
	}
	wg.Wait()

	select {
	case err := <-chErr:
		return err
	default:
		return nil
	}
}
