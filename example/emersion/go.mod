module github.com/isayme/afero-webdav/example/emersion

go 1.26.2

require (
	github.com/emersion/go-webdav v0.4.0
	github.com/isayme/afero-webdav v0.0.0
)

require (
	github.com/spf13/afero v1.15.0 // indirect
	golang.org/x/text v0.37.0 // indirect
)

replace github.com/isayme/afero-webdav => ../..
