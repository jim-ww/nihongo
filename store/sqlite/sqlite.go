package sqlite

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/gojp/kana"
	"github.com/jim-ww/nihongo/store"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

const schema = `
CREATE VIRTUAL TABLE IF NOT EXISTS fts_dict USING fts5(
    expression,
    reading,
    reading_romaji,
    definitions,
    examples,
    pos,
    definition_tags,
    term_tags,
    sequence,
    score,
    tokenize=unicode61,
    prefix='2 3 4'
)`

func NewDB(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 10000;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA cache_size = -64000;",
		"PRAGMA foreign_keys = ON;",
	}

	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, err
		}
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return &DB{DB: db}, nil
}

func (d *DB) InsertFtsDictBatch(bank []store.FtsDict) error {
	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
        INSERT INTO fts_dict
        (expression, reading, reading_romaji, definitions, examples, pos, definition_tags, term_tags, sequence, score)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, entry := range bank {
		defsJoined := joinDefinitions(entry.Definitions)
		examplesJoined := joinDefinitions(entry.Examples)
		posJoined := joinDefinitions(entry.Pos)

		_, err := stmt.Exec(
			entry.Expression,
			entry.Reading,
			entry.ReadingRomaji,
			defsJoined,
			examplesJoined,
			posJoined,
			entry.DefinitionTags,
			entry.TermTags,
			entry.Sequence,
			entry.Score,
		)
		if err != nil {
			return fmt.Errorf("insert: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (d *DB) HasAtLeastOneEntry() (bool, error) {
	var has bool
	err := d.QueryRow("SELECT COUNT(*) > 0 FROM fts_dict").Scan(&has)
	return has, err
}

func (d *DB) FindEntryByID(id int) (store.FtsDict, error) {
	const q = `
        SELECT rowid, expression, reading, reading_romaji, definitions, examples, pos,
               definition_tags, term_tags, score, sequence
        FROM fts_dict
        WHERE rowid = ?
    `

	var dict store.FtsDict
	var defs, examples, pos string

	err := d.QueryRow(q, id).Scan(
		&dict.RowID, &dict.Expression, &dict.Reading, &dict.ReadingRomaji,
		&defs, &examples, &pos,
		&dict.DefinitionTags, &dict.TermTags, &dict.Score, &dict.Sequence,
	)

	if err == nil {
		dict.Definitions = splitDefinitions(defs)
		dict.Examples = splitDefinitions(examples)
		dict.Pos = splitDefinitions(pos)
	}

	return dict, err
}

// - kanji -> search in expression 'kana'+'*'
// - romaji (if latin and no -e flag) -> search 'input'+'*' in reading_romaji, max priority on equality
// - English (if latin + -e flag) -> search in definitions, how?
func (d *DB) Search(input string, limit int, isEnglish bool) ([]store.FtsDict, error) {
	originalInput := input
	query := input

	// Normalization
	// - hiragana/katakana -> convert to romaji
	if kana.IsHiragana(input) || kana.IsKatakana(input) {
		query = kana.KanaToRomaji(input)
	} else if kana.IsLatin(input) && !isEnglish {
		// - romaji
		query = kana.NormalizeRomaji(input)
	}

	var where, orderBy string
	var args []any

	switch {
	case kana.IsLatin(input) && isEnglish:
		// English search
		where = `definitions MATCH ?`
		orderBy = `
			CASE
				WHEN definitions LIKE '% ' || ? || ' %'
				  OR definitions LIKE ? || ' %'
				  OR definitions LIKE '% ' || ?
				THEN 10000
				ELSE 0
			END DESC,
			score DESC`
		args = []any{query + "*", query, query, query}

	default:
		// Japanese / Romaji search
		exprPrefix := originalInput + "*"
		romajiClean := strings.ReplaceAll(query, "-", "")
		romajiPrefix := romajiClean + "*"
		where = `expression MATCH ? OR reading_romaji MATCH ?`
		orderBy = `score DESC,
		           CASE WHEN expression = ? THEN 4500
		                WHEN reading_romaji = ? THEN 4000
		                ELSE 0 END DESC,
		           bm25(fts_dict, 20, 3, 38, 3, 1, 1, 1, 1, 1, 1) DESC`
		args = []any{exprPrefix, romajiPrefix, originalInput, query}
	}

	q := fmt.Sprintf(`
        SELECT rowid, expression, reading, reading_romaji, definitions, examples, pos,
               definition_tags, term_tags, score, sequence
        FROM fts_dict
        WHERE %s
        ORDER BY %s
        LIMIT ?`, where, orderBy)

	args = append(args, limit)

	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var entries []store.FtsDict
	for rows.Next() {
		var dict store.FtsDict
		var defs, examples, posStr string

		if err := rows.Scan(
			&dict.RowID, &dict.Expression, &dict.Reading, &dict.ReadingRomaji,
			&defs, &examples, &posStr,
			&dict.DefinitionTags, &dict.TermTags, &dict.Score, &dict.Sequence,
		); err != nil {
			return nil, err
		}

		dict.Definitions = splitDefinitions(defs)
		dict.Examples = splitDefinitions(examples)
		dict.Pos = splitDefinitions(posStr)

		entries = append(entries, dict)
	}
	return entries, nil
}

func (d *DB) Close() error {
	return d.DB.Close()
}

func joinDefinitions(defs []string) string {
	if len(defs) == 0 {
		return ""
	}
	return strings.Join(defs, "\n\n")
}

func splitDefinitions(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n\n")
}
