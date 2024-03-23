@echo off

go build -o ./bin/server.exe ./pkg/server 
go build -o ./bin/client.exe ./pkg/client