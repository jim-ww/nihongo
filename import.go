package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jim-ww/nihongo/store"
	"golang.org/x/sync/errgroup"
)

func importDict(st store.Store, zipReader io.ReaderAt, size int64, appData string) error {
	zr, err := zip.NewReader(zipReader, size)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}

	batchChan := make(chan []store.FtsDict, 8)

	var totalCount int64
	var wg sync.WaitGroup

	wg.Go(func() {
		for bank := range batchChan {
			if err := st.InsertFtsDictBatch(bank); err != nil {
				log.Printf("insert batch error: %v", err)
			}
			atomic.AddInt64(&totalCount, int64(len(bank)))
		}
	})

	g := new(errgroup.Group)
	g.SetLimit(runtime.NumCPU() * 4)
	start := time.Now()

	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "term_bank_") || !strings.HasSuffix(f.Name, ".json") {
			if f.Name == "index.json" {
				rc, err := f.Open()
				if err != nil {
					return err
				}
				defer rc.Close()
				out, err := os.Create(filepath.Join(appData, dictInfoFileName))
				if err != nil {
					return err
				}
				defer out.Close()
				_, err = io.Copy(out, rc)
			}
			continue
		}
		file := f
		g.Go(func() error {
			var bank [][]any

			r, err := f.Open()
			if err != nil {
				return fmt.Errorf("failed to open zip file: %w", err)
			}
			defer r.Close()

			if err := sonic.ConfigDefault.NewDecoder(r).Decode(&bank); err != nil {
				return fmt.Errorf("failed to unmarshal %s: %w", file.Name, err)
			}

			batch := make([]store.FtsDict, 0, len(bank))
			for i := range bank {
				batch = append(batch, convertToFtsDict(bank[i]))
			}

			batchChan <- batch
			fmt.Printf("\rParsed %s (%d entries)", file.Name, len(bank))
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		close(batchChan)
		wg.Wait()
		return err
	}
	close(batchChan)
	wg.Wait()
	fmt.Printf("\rSuccessfully indexed %d entries in total. Time took: %s\n", totalCount, time.Since(start))
	return nil
}

func getDataHome() string {
	expandTilde := func(path string) string {
		if !strings.HasPrefix(path, "~") {
			return path
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		if path == "~" {
			return home
		}
		return filepath.Join(home, path[2:])
	}

	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return expandTilde(dir)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	switch runtime.GOOS {
	case "linux", "freebsd", "openbsd", "netbsd":
		return filepath.Join(home, ".local", "share")

	case "darwin":
		return filepath.Join(home, "Library", "Application Support")

	case "windows":
		if dir := os.Getenv("APPDATA"); dir != "" {
			return dir
		}
		return filepath.Join(home, "AppData", "Roaming")

	default:
		return filepath.Join(home, ".local", "share")
	}
}
