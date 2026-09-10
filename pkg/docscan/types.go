package docscan

// Document represents an ingested architectural or domain documentation file.
type Document struct {
	RepoPath     string `json:"repo_path"`
	RelativePath string `json:"relative_path"`
	Title        string `json:"title"`
	Content      string `json:"content"`
}

// ScanResult contains discovered documentation files and non-fatal warnings encountered during scanning.
type ScanResult struct {
	Documents []Document `json:"documents"`
	Warnings  []string   `json:"warnings"`
}

// Config defines thresholds and options for scanning and collation.
type Config struct {
	MaxTotalBytes int
	MaxDocBytes   int
	WarnFunc      func(warning string)
}

const (
	// DefaultMaxTotalBytes limits total collated documentation context (~15k tokens).
	DefaultMaxTotalBytes = 60000
	// DefaultMaxDocBytes limits individual document size before truncation (~3k tokens).
	DefaultMaxDocBytes = 15000
)

// DefaultConfig returns standard thresholds for doc scanning and collation.
func DefaultConfig() Config {
	return Config{
		MaxTotalBytes: DefaultMaxTotalBytes,
		MaxDocBytes:   DefaultMaxDocBytes,
	}
}
