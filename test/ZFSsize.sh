#!/bin/bash
#
echo -e "\nZFS list"
zfs list tank0/nntp-history -r

echo -e "\nZFS compressratio"
zfs get compressratio tank0/nntp-history -r

echo -e "\ndiskunits: history"
echo "du -b history/: $(du -b /tank0/nntp-history/history/)"
echo "du -bs history/: $(du -bs /tank0/nntp-history/history/)"
echo "du -hs history/: $(du -hs /tank0/nntp-history/history/)"
echo -e "du --apparent-size history/:\n$(du --apparent-size /tank0/nntp-history/history/)"
echo -e "du -h --apparent-size history/:\n$(du -h --apparent-size /tank0/nntp-history/history/)"

echo -e "\ndiskunits hashdb"
echo "du -b hashdb/: $(du -b /tank0/nntp-history/hashdb/)"
echo "du -bs hashdb/: $(du -bs /tank0/nntp-history/hashdb/)"
echo "du -hs hashdb/: $(du -hs /tank0/nntp-history/hashdb/)"
echo -e "du --apparent-size hashdb/:\n$(du --apparent-size /tank0/nntp-history/hashdb/)"
echo -e "du -h --apparent-size hashdb/:\n$(du -h --apparent-size /tank0/nntp-history/hashdb/)"

echo -e "\ndiskunits .dat files"
echo -e "du -b history/*.dat hashdb/*.*:\n$(du -b history/*.dat)\n$(du -b hashdb/*.*)\n"
echo -e "du -bs history/*.dat hashdb/*.*:\n$(du -bs history/*.dat)\n$(du -bs hashdb/*.*)\n"
echo -e "du -hs history/*.dat hashdb/*.*:\n$(du -hs history/*.dat)\n$(du -hs hashdb/*.*)\n"
echo -e "du --apparent-size hashdb history/*.dat hashdb/*.*:\n$(du --apparent-size history/*.dat)\n$(du --apparent-size hashdb/*.*)\n"
echo -e "du -h --apparent-size hashdb history/*.dat hashdb/*.*:\n$(du -h --apparent-size history/*.dat)\n$(du -h --apparent-size hashdb/*.*)\n"



