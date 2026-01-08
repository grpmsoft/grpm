package application

// DTOs (Data Transfer Objects) for Application Service layer
// Following DDD pattern: DTOs are simple data structures for transferring data
// across layer boundaries (Application → Interface/gRPC)

// ResolutionResult represents the result of dependency resolution
type ResolutionResult struct {
	Success           bool
	PackagesToInstall map[string]string // package name -> version
	PackagesToUpdate  []string
	Conflicts         []string
	TotalSize         int64 // Total download size in bytes
	Error             string
}

// PackageInfo represents detailed package information
type PackageInfo struct {
	Name         string
	Version      string
	Slot         string
	Subslot      string
	Description  string
	Homepage     string
	License      string
	UseFlags     []string
	Dependencies []string
	Installed    bool
}

// InstallProgress represents installation progress events
type InstallProgress struct {
	Stage     string // "resolving", "downloading", "compiling", "installing"
	Message   string
	Percent   int
	Timestamp int64
}

// SearchResult represents search results
type SearchResult struct {
	Packages   []*PackageInfo
	TotalCount int
}

// RemovalResult represents package removal result
type RemovalResult struct {
	Success        bool
	RemovedPackage string
	Message        string
}

// UpdateResult represents system update result
type UpdateResult struct {
	Success         bool
	UpdatedPackages []string
	Message         string
}
