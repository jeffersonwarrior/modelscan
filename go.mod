module github.com/jeffersonwarrior/modelscan

go 1.25.5

require (
	github.com/google/uuid v1.6.0
	github.com/jeffersonwarrior/modelscan/internal/admin v0.0.0-00010101000000-000000000000
	github.com/jeffersonwarrior/modelscan/internal/config v0.0.0-00010101000000-000000000000
	github.com/jeffersonwarrior/modelscan/internal/discovery v0.0.0-00010101000000-000000000000
	github.com/jeffersonwarrior/modelscan/internal/generator v0.0.0-00010101000000-000000000000
	github.com/jeffersonwarrior/modelscan/internal/keymanager v0.0.0-00010101000000-000000000000
	github.com/mattn/go-sqlite3 v1.14.32
	github.com/sashabaranov/go-openai v1.41.2
	github.com/spf13/cobra v1.10.2
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/jeffersonwarrior/modelscan/internal/admin => ./internal/admin

replace github.com/jeffersonwarrior/modelscan/internal/config => ./internal/config

replace github.com/jeffersonwarrior/modelscan/internal/database => ./internal/database

replace github.com/jeffersonwarrior/modelscan/internal/discovery => ./internal/discovery

replace github.com/jeffersonwarrior/modelscan/internal/generator => ./internal/generator

replace github.com/jeffersonwarrior/modelscan/internal/keymanager => ./internal/keymanager
