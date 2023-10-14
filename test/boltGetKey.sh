#/bin/bash
test -z "$3" && echo "usage: $0 hashdb/history.dat.hash.f 0d 784ae1" && exit 1
DBFILE="$1"
BUCKET="$2"
KEY="$3"
test ! -f "$DBFILE" && echo "DBFILE not found" && exit 1
bbolt=/GO/go/bin/bbolt
CMD="$bbolt get $DBFILE $BUCKET $KEY"
VAL=$($CMD|tr ',' '\n')
test -z "$VAL" && echo "no values in DB=$DBFILE BUCKET=$BUCKET KEY=$KEY" && exit 42
echo "DB=$DBFILE BUCKET=$BUCKET KEY=$KEY values:"
for x in $VAL; do
 test -z "$x" && continue
 decval=$(printf "%d" "0x$x")
 echo "0x$x = $decval"
 ./nntp-history-test -getHL=$decval
done
