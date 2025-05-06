rm -fr dist *.log app/dist
rm -fr node_modules
go clean
rm -f dixbatch dixcron dixfe dixlive dixmgr dixfil *_cli
rm -fr bin

