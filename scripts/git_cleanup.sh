go clean
# generated
rm -fr bin dist *.log app/dist
rm -fr node_modules
# binaries in wrong places
rm -f dixbatch dixcron dixfe dixlive dixmgr dixfil *_cli
# do not remove the .scss
rm app/dix-large.* app/dix.css*

