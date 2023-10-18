./build.sh || exit $?
rm /tank0/nntp-history/*/*.* -vr
ln -sfv /tank0/nntp-history/hashdb .
ln -sfv /tank0/nntp-history/history .
rm -v *.out
