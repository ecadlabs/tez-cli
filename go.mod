module github.com/ecadlabs/tez

go 1.12

require (
	github.com/ecadlabs/go-tezos v0.0.0-20190909142034-0c0a4dddb29b
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/kr/pretty v0.1.0 // indirect
	github.com/logrusorgru/aurora v0.0.0-20190803045625-94edacc10f9b
	github.com/mattn/go-isatty v0.0.9
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.5
	golang.org/x/sys v0.0.0-20190909082730-f460065e899a // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/yaml.v3 v3.0.0-20190905181640-827449938966
)

//replace github.com/ecadlabs/go-tezos => ../go-tezos
