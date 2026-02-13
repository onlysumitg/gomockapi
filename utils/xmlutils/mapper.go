package xmlutils

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

func XmlToMap(r io.Reader) (map[string]any) {
	// result
	//m := make(map[string]string)
	// the current value stack
	//values := make([]string, 0)
	// parser

	elementCountMap := make(map[string]int)

	xmlMap := make(map[string]any)

	p := xml.NewDecoder(r)

	key := "$"
	masterkey := "$"
	for token, err := p.Token(); err == nil; token, err = p.Token() {

		fmt.Println("------------------------", elementCountMap)

		//fmt.Println("token", token)

		switch t := token.(type) {
		case xml.CharData:
			xmlMap[key] = string([]byte(t))
			// push

			//fmt.Println("CharData values::", string([]byte(t)))
		case xml.EndElement:
			//fmt.Println("StartElement values::", t.Name.Local, t.Attr)
			element_name := ""

			if t.Name.Space == "" {
				element_name = "_" + t.Name.Local

			} else {
				element_name = "_" + t.Name.Space + "_" + t.Name.Local

			}

			element_name_unindexed := element_name
			count, found := elementCountMap[masterkey]
			if found && count-1 > 0 {

				element_name = fmt.Sprintf("%s[%d]", element_name, count-1)

			}

			if strings.HasSuffix(key, element_name) {
				key = strings.TrimSuffix(key, element_name)
			} else {
				fmt.Println(">>>>>>>>>>  error >>>", key, element_name)
			}

			if strings.HasSuffix(masterkey, element_name) {
				masterkey = strings.TrimSuffix(masterkey, element_name_unindexed)
			} else {
				fmt.Println(">>>>>>>>>>  error >>>", key, element_name)
			}

			fmt.Println(">>>>>>>>>>  EndElement >>>", key, " :: ", element_name)

		case xml.StartElement:
			//fmt.Println("EndElement values::", t.Name.Local)
			element_name := ""

			if t.Name.Space == "" {
				element_name = "_" + t.Name.Local

			} else {
				element_name = "_" + t.Name.Space + "_" + t.Name.Local

			}

			key = key + element_name
			masterkey = masterkey + element_name
			e := key

			count, found := elementCountMap[e]
			if found {
				key = fmt.Sprintf("%s[%d]", key, count)
				elementCountMap[e] = count + 1
			} else {
				elementCountMap[e] = 1
			}

			fmt.Println(">>>>>>>>>>  StartElement >>>", key, " :: ", element_name)

			for _, a := range t.Attr {
				attribute := key + "_*ATTR_" + a.Name.Space + "_" + a.Name.Local
				fmt.Println(">>>>>>>>>>  StartElement attribute >>>", attribute)

				xmlMap[attribute] = a.Value

			}

		default:
			// fmt.Println(" type>>>>> ", reflect.TypeOf(token))
			// fmt.Printf(" value>>>>> %v", token)
			// fmt.Println(" t>>>>> ", t)

		}
	}
	// done
	return xmlMap
}
