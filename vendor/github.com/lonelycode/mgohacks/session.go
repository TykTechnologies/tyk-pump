// contents copied from github.com/go-mgo/mgo@3569c88678d88179dcbd68d02ab081cbca3cd4d0

package mgohacks

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/mgo.v2/bson"
)

type indexKeyInfo struct {
	name    string
	key     bson.D
	weights bson.D
}

func parseIndexKey(key []string) (*indexKeyInfo, error) {
	var keyInfo indexKeyInfo
	isText := false
	var order interface{}
	for _, field := range key {
		raw := field
		if keyInfo.name != "" {
			keyInfo.name += "_"
		}
		var kind string
		if field != "" {
			if field[0] == '$' {
				if c := strings.Index(field, ":"); c > 1 && c < len(field)-1 {
					kind = field[1:c]
					field = field[c+1:]
					keyInfo.name += field + "_" + kind
				} else {
					field = "\x00"
				}
			}
			switch field[0] {
			case 0:
				// Logic above failed. Reset and error.
				field = ""
			case '@':
				order = "2d"
				field = field[1:]
				// The shell used to render this field as key_ instead of key_2d,
				// and mgo followed suit. This has been fixed in recent server
				// releases, and mgo followed as well.
				keyInfo.name += field + "_2d"
			case '-':
				order = -1
				field = field[1:]
				keyInfo.name += field + "_-1"
			case '+':
				field = field[1:]
				fallthrough
			default:
				if kind == "" {
					order = 1
					keyInfo.name += field + "_1"
				} else {
					order = kind
				}
			}
		}
		if field == "" || kind != "" && order != kind {
			return nil, fmt.Errorf(`invalid index key: want "[$<kind>:][-]<field name>", got %q`, raw)
		}
		if kind == "text" {
			if !isText {
				keyInfo.key = append(keyInfo.key, bson.DocElem{"_fts", "text"}, bson.DocElem{"_ftsx", 1})
				isText = true
			}
			keyInfo.weights = append(keyInfo.weights, bson.DocElem{field, 1})
		} else {
			keyInfo.key = append(keyInfo.key, bson.DocElem{field, order})
		}
	}
	if keyInfo.name == "" {
		return nil, errors.New("invalid index key: no fields provided")
	}
	return &keyInfo, nil
}
