package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jim-ww/nihongo/store"
	"github.com/jim-ww/nihongo/store/sqlite"
	"github.com/ktr0731/go-fuzzyfinder"
)

const (
	appName            = "nihongo"
	version            = ""
	dictInfoFileName   = "dictionary.json"
	dbFileName         = "nihongo.db"
	msgNoDictInstalled = "You do not have any dictionaries installed. Use -download or -import command first."
)

var romajiFlag = flag.Bool("r", false, "show romaji in list of results")

func main() {
	importFlag := flag.String("import", "", "import yomitan's dictionary zip file (path)")
	versionFlag := flag.Bool("version", false, fmt.Sprintf("print %s app version", appName))
	dictDownloadURL := flag.String("dictDownloadUrl", "https://github.com/stephenmk/stephenmk.github.io/releases/latest/download/jitendex-yomitan.zip", "where to download zip dictionary from. By default, downloads latest version of jitindex: https://jitendex.org/")
	downloadFlag := flag.Bool("download", false, "download yomitan-compatible dictionary from -dictDownloadUrl")
	infoFlag := flag.Bool("info", false, "show info about installed dictionary")
	englishFlag := flag.Bool("e", false, "treat all latin input as English (not romaji)")
	truncateFlag := flag.Bool("truncate", true, "remove all entries, before importing from new dictionary")
	limitFlag := flag.Int("l", 20, "limit number of search results to show")
	clearCacheFlag := flag.Bool("clearCache", false, "clear cached downloaded dictionaries")
	fzfFlag := flag.Bool("fzf", true, "enable fuzzy entries selection")
	showID := flag.Bool("showID", true, "show IDs of entries. Only works when -fzf=true")
	idFlag := flag.Int("id", -1, "print entry by its ID")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s: A Japanese Dictionary CLI Tool\n", appName)
		fmt.Fprintf(os.Stderr, "Usage: %s %s\n\n", appName, "日本語")
		fmt.Fprintf(os.Stderr, "Other Commands of %s:\n", appName)
		flag.PrintDefaults()
	}

	flag.Parse()
	appData := filepath.Join(getDataHome(), appName)
	dbPath := filepath.Join(appData, dbFileName)

	if (*downloadFlag || strings.TrimSpace(*importFlag) != "") && *truncateFlag {
		if err := os.RemoveAll(appData); err != nil {
			log.Fatalf("Failed to remove old app data files, before importing new dictionary: %v", err)
		}
	}

	if err := os.MkdirAll(appData, 0o755); err != nil {
		log.Fatal(err)
	}
	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to open/initialize db file: %v", err)
	}
	store := store.Store(db)
	defer func() {
		if err := store.Close(); err != nil {
			fmt.Println("WARNING:", err)
		}
	}()
	hasAtLeastOneEntry, err := store.HasAtLeastOneEntry()
	if err != nil {
		log.Fatalf("Failed to check if at least one entry exists: %v", err)
	}

	switch {
	case *downloadFlag:
		fmt.Println("Downloading dictionary file...", *dictDownloadURL)
		tmpDir := filepath.Join(os.TempDir(), appName)
		if err := os.MkdirAll(tmpDir, 0o755); err != nil {
			log.Fatalf("Failed to create temp dir: %v", err)
		}
		tmpFile := filepath.Join(tmpDir, fmt.Sprintf("jitendex-release_%s.zip", time.Now().Format("2006-01-02")))

		stat, err := os.Stat(tmpFile)
		var f *os.File
		if err != nil {
			f, err = os.Create(tmpFile)
			if err != nil {
				log.Fatalf("Failed to create temp file: %v", err)
			}
			defer f.Close()
			resp, err := http.Get(*dictDownloadURL)
			if err != nil {
				log.Fatalf("failed to download %s: %v", *dictDownloadURL, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				log.Fatalf("bad status code downloading %s: %d", *dictDownloadURL, resp.StatusCode)
			}
			if _, err = io.Copy(f, resp.Body); err != nil {
				log.Fatalf("failed to copy downloaded file: %v", err)
			}
			stat, err = os.Stat(tmpFile)
			if err != nil {
				log.Fatalf("Failed to get size of a file: %v", err)
			}
			fmt.Println("Successfully downloaded dictionary. Importing entries to database...")
		} else {
			f, err = os.Open(tmpFile)
			if err != nil {
				log.Fatalf("Failed to open cached temp file: %v", err)
			}
			defer f.Close()
			fmt.Println("Using cached downloaded dictionary. Importing entries to database...")
		}

		if err := importDict(store, f, stat.Size(), appData); err != nil {
			log.Fatalf("Failed to create dictionary index: %v", err)
		}
		os.Exit(0)
	case *infoFlag:
		f, err := os.ReadFile(filepath.Join(appData, dictInfoFileName))
		if err != nil {
			log.Fatal(msgNoDictInstalled)
		}
		info := map[string]any{}
		if err := sonic.Unmarshal(f, &info); err != nil {
			log.Fatalf("Invalid dictionary info file: %v", err)
		}
		for k, v := range info {
			fmt.Printf("%s: %s\n", k, fmt.Sprint(v))
		}
	case *idFlag != -1:
		entry, err := store.FindEntryByID(*idFlag)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				log.Fatal("Entry not found")
			}
			log.Fatalf("Failed to find entry: %v", err)
		}
		fmt.Println(formatEntry(entry))
	case *clearCacheFlag:
		if err := os.RemoveAll(filepath.Join(os.TempDir(), appName)); err != nil {
			log.Fatalf("Failed to clear app cache: %v", err)
		}
		fmt.Println("Successfully cleared cache")
		os.Exit(0)
	case *versionFlag:
		osArch := runtime.GOOS + "/" + runtime.GOARCH
		fmt.Printf("%s %s\n", version, osArch)
		os.Exit(0)
	case strings.TrimSpace(*importFlag) != "":
		f, err := os.Open(*importFlag)
		if err != nil {
			log.Fatalf("Failed to open cached temp file: %v", err)
		}
		stat, err := os.Stat(*importFlag)
		if err != nil {
			log.Fatalf("Failed to open import file: %v", err)
		}
		if err := importDict(store, f, stat.Size(), appData); err != nil {
			log.Fatalf("Failed to create dictionary index: %v", err)
		}
		os.Exit(0)

	default:
		if len(os.Args) < 2 {
			flag.Usage()
			return
		}

		if !hasAtLeastOneEntry && !*downloadFlag {
			fmt.Println(msgNoDictInstalled)
			os.Exit(0)
		}

		input := os.Args[len(os.Args)-1]
		if strings.HasPrefix(input, "-") {
			log.Fatal("flags must come first before search term")
		}

		results, err := store.Search(input, *limitFlag, *englishFlag)
		if err != nil {
			log.Fatalf("Search failed: %v", err)
		}

		if len(results) == 0 {
			fmt.Printf("No results found for: %s\n", input)
			return
		}

		if !*fzfFlag {
			for _, r := range results {
				if *showID {
					fmt.Printf("%d: %s\n", r.RowID, formatList(r))
				} else {
					fmt.Println(formatList(r))
				}
			}
			os.Exit(0)
		}

		selected, err := fuzzyfinder.Find(results, func(i int) string { return formatList(results[i]) })
		if err != nil {
			if errors.Is(err, fuzzyfinder.ErrAbort) {
				os.Exit(0)
			}
			log.Fatalf("Failed to perform search: %v", err)
		}

		fmt.Println(formatEntry(results[selected]))
	}
}
