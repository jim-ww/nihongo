package store

type FtsDict struct {
	RowID          int      `db:"rowid"`
	Expression     string   `db:"expression" json:"expression"`
	Reading        string   `db:"reading" json:"reading"`
	ReadingRomaji  string   `db:"reading_romaji" json:"reading_romaji"`
	Definitions    []string `db:"definitions" json:"definitions"`
	Examples       []string `db:"examples" json:"examples"`
	Pos            []string `db:"pos" json:"pos"`
	DefinitionTags string   `db:"definition_tags" json:"definition_tags"`
	TermTags       string   `db:"term_tags" json:"term_tags"`
	Score          float64  `db:"score" json:"score"`
	Sequence       string   `db:"sequence" json:"sequence"`
}

type Store interface {
	InsertFtsDictBatch(bank []FtsDict) error
	FindEntryByID(id int) (FtsDict, error)
	Search(input string, limit int, isEnglish bool) ([]FtsDict, error)
	HasAtLeastOneEntry() (bool, error)
	Close() error
}
