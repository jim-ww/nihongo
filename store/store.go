package store

// Example is a single JP/EN example sentence pair attached to a Sense.
type Example struct {
	JP string `json:"jp"`
	EN string `json:"en"`
}

// Sense is one numbered meaning of a term: its gloss, any notes or
// cross-references that qualify it, and the example sentences illustrating it.
type Sense struct {
	Gloss    string    `json:"gloss"`
	Notes    []string  `json:"notes,omitempty"`
	Xrefs    []string  `json:"xrefs,omitempty"`
	Examples []Example `json:"examples,omitempty"`
}

// SenseGroup is a set of senses that share the same part-of-speech, mirroring
// how Yomitan dictionaries group meanings under a POS heading.
type SenseGroup struct {
	Pos    []string `json:"pos,omitempty"`
	Senses []Sense  `json:"senses"`
}

type FtsDict struct {
	RowID          int          `db:"rowid"`
	Expression     string       `db:"expression" json:"expression"`
	Reading        string       `db:"reading" json:"reading"`
	ReadingRomaji  string       `db:"reading_romaji" json:"reading_romaji"`
	Definitions    []string     `db:"definitions" json:"definitions"`
	Examples       []string     `db:"examples" json:"examples"`
	Pos            []string     `db:"pos" json:"pos"`
	Groups         []SenseGroup `db:"groups" json:"groups"`
	Forms          []string     `db:"forms" json:"forms"`
	DefinitionTags string       `db:"definition_tags" json:"definition_tags"`
	TermTags       string       `db:"term_tags" json:"term_tags"`
	Score          float64      `db:"score" json:"score"`
	Sequence       string       `db:"sequence" json:"sequence"`
}

type Store interface {
	InsertFtsDictBatch(bank []FtsDict) error
	FindEntryByID(id int) (FtsDict, error)
	Search(input string, limit int, isEnglish bool) ([]FtsDict, error)
	HasAtLeastOneEntry() (bool, error)
	Close() error
}
