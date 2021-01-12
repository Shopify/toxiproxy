set GOOS=%1
set GOARCH=amd64
go build -o toxiproxy-server-%1 ./cmd
go build -o toxiproxy-cli-%1 ./cli