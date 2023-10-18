package history

/*
 *  dump() function can be used in bbolt:node.go to debug writes.
 *  add n.dump() to the end of write() function in bbolt:node.go
 *
 *

// mod@date: 2023-10-18_12:00
var print_header bool = false
var printSpilled bool = true
var printUnbalanced bool = true
var print_isLeaf bool = false
var print_noLeaf bool = true
var print_item   bool = false
var print_dumped bool = false
var debugsleep0 = 0 * time.Millisecond // sleeps after printing: header
var debugsleep1 = 1000 * time.Millisecond // sleeps after printing: if spilled or unbalanced
var debugsleep2 = 0 * time.Millisecond // sleeps if is Leaf
var debugsleep3 = 1000 * time.Millisecond // sleeps if no Leaf

// dump writes the contents of the node to STDERR for debugging purposes.
func (n *node) dump() {
	// prints node header.
	var typ = "branch"
	if n.isLeaf {
		typ = "leaf"
	}
	now := time.Now().UnixNano()

	if print_header {
		log.Printf("NEW id=%d\n [NODE %d {type=%s inodes=%d}]\n n='%#v'", now, n.pgid, typ, len(n.inodes), n)
		time.Sleep(debugsleep0)
	}

	if printSpilled && n.spilled {
		log.Printf("!!! id=%d spil=true [NODE %d {type=%s inodes=%d}]", now, n.pgid, typ, len(n.inodes))
		time.Sleep(debugsleep1)
	} else
	if printUnbalanced && n.unbalanced {
		log.Printf("!!! id=%d unba=true [NODE %d {type=%s inodes=%d}]", now, n.pgid, typ, len(n.inodes))
		time.Sleep(debugsleep1)
	}

	for _, item := range n.inodes {
		if n.isLeaf {
			if print_isLeaf {
				if print_item {
					log.Printf("+++ id=%d write: isLeaf=1 key='%08x' item='%#v' pgid=%d key='%s' v='%s'", now, item.Key, item, n.pgid, string(item.Key()), string(item.Value()))
				} else {
					log.Printf("+++ id=%d write: isLeaf=1 pgid=%d buk=%s key='%s' v='%s'", now, n.pgid, "buk?", string(item.Key()), string(item.Value()))
				}
				time.Sleep(debugsleep2)
			}
		} else {
			if print_noLeaf {
				if print_item {
					log.Printf("+++ id=%d write: isLeaf=0 key='%08x' item='%#v' pgid=%d key='%s' v='%s'", now, item.Key, item, n.pgid, string(item.Key()), string(item.Value()))
				} else {
					log.Printf("+++ id=%d write: isLeaf=0 pgid=%d buk=%s key='%s' v='%s'", now, n.pgid, "buk?", string(item.Key()), string(item.Value()))
				}
				time.Sleep(debugsleep3)
			}

			//log.Printf("+B %08x -> pgid=%d", trunc(item.key, 4), item.pgid)
		}
	}
	if print_dumped {
		log.Printf("END id=%d dumped", now)
	}
} // end func dump


*
*
*
*/
