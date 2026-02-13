package main

import (
	"fmt"
	"os"
	"reflect"

	"github.com/rbretecher/go-postman-collection"
)

// -----------------------------------------------------------------------
// https://learning.postman.com/collection-format/getting-started/overview/
// -----------------------------------------------------------------------

func main() {
	filename := "./uploads/paypal.json"
	ReadPostmantJson(filename)
}

// -----------------------------------------------------------------------
//
// -----------------------------------------------------------------------

func ReadPostmantJson(filename string) {

	// https://learning.postman.com/collection-format/getting-started/overview/

	fmt.Println(">>>filename?>>", filename)
	file, err := os.Open(filename)
	defer file.Close()

	if err != nil {
		fmt.Println("error1", err, filename)
	}

	c, err := postman.ParseCollection(file)
	if err != nil {
		fmt.Println("error2", err, filename)
	}

	fmt.Println("Postman connection info ==== start")
	fmt.Println("c.Info.Name", c.Info.Name)
	fmt.Println("c.Info.Schema", c.Info.Schema)
	fmt.Println("c.Info.Version", c.Info.Version)
	fmt.Println("c.Info.Description.Content", c.Info.Description.Content)
	fmt.Println("c.Info.Description.Type", c.Info.Description.Type)
	fmt.Println("c.Info.Description.Version", c.Info.Description.Version)
	fmt.Println("Postman connection info ==== ======================== =====================end")

	for _, v := range c.Variables {
		fmt.Println("Variable::", v.Key, v.Value)
	}

	// for _, item := range c.Items {

	// 	ProcessPostmanItem(item, 0)
	// }
}

// -----------------------------------------------------------------------
//
// -----------------------------------------------------------------------

func ProcessPostmanItem(item *postman.Items, level int) {

	fmt.Println("======================Start ITEM ================", level)
	fmt.Println("Postman item name", item.Name)
	fmt.Println("Postman item Description", item.Description)
	// fmt.Println("Postman item Variables", item.Variables)
	// fmt.Println("Postman item Events", item.Events)
	// fmt.Println("Postman item ProtocolProfileBehavior", item.ProtocolProfileBehavior)
	// fmt.Println("Postman item ID", item.ID)

	if item.Request != nil {
		// fmt.Println("Postman item Request URL", item.Request.URL)
		// fmt.Println("Postman item Request.Auth", item.Request.Auth)
		// fmt.Println("Postman item Request.Method", item.Request.Method)
		// fmt.Println("Postman item Request.Description", item.Request.Description)

		// if item.Request.Header != nil {
		// 	headerList := postman.HeaderList{item.Request.Header}
		// 	hj, _ := headerList.MarshalJSON()
		// 	fmt.Println("Postman item Request.Header", hj)
		// }

		//ProcessBody(item.Request.Body)
		//fmt.Println("Postman item Request.Body", item.Request.Body)
		//fmt.Println("Postman item Request.Body", item.Request.Body.Options.Raw.Language)

	}

	if item.Responses != nil {

		fmt.Println("Postman item Responses", item.Responses)
	}
	// fmt.Println("Postman item child Auth", item.Auth)
	// fmt.Println("Postman item child items", item.Items)

	for _, i := range item.Items {
		ProcessPostmanItem(i, level+1)
	}

	fmt.Println("======================END ITEM ================", level)

}

// -----------------------------------------------------------------------
//
// -----------------------------------------------------------------------
func ProcessBody(body *postman.Body) {
	if body == nil {
		return
	}

	switch body.Mode {
	case "raw":
		fmt.Println("raw")
	case "urlencoded":
		fmt.Println("urlencoded", body.URLEncoded)

		// x, err := json.Marshal(body.URLEncoded)
		// if err != nil {
		// 	fmt.Println("ERROR***********", err)

		// } else {
		// 	fmt.Println("JSON***********", string(x))

		// 	j := make([]map[string]any, 0)
		// 	err := json.Unmarshal(x, &j)
		// 	if err != nil {
		// 		fmt.Println("        NOT JSON :::::", err.Error())
		// 	} else {

		// 		fmt.Println("        2 JSON :::::", j)
		// 	}

		// }

		// if IsList(body.URLEncoded) {
		// 	fmt.Println("IS LIST ***********")

		// 	str, ok := body.URLEncoded.(string)
		// 	if ok {
		// 		fmt.Println("STRING IS OK **** ", str)
		// 	} else {
		// 		fmt.Println("NOT STRING IS OK **** ")

		// 	}

		// 	s := reflect.ValueOf(body.URLEncoded)
		// 	for i := 0; i < s.Len(); i++ {

		// 		ss, ok := s.Index(i).(string)
		// 		if !ok {

		// 		}
		// 		// j := make(map[string]any)
		// 		// err := json.Unmarshal(, &j)
		// 		// if err != nil {
		// 		// 	fmt.Println("        NOT JSON :::::", err.Error())

		// 		// } else {
		// 		// 	fmt.Println("        JSON :::::", j)
		// 		// }
		// 		//fmt.Println(s.Index(i))
		// 	}

		// } else {
		// 	fmt.Println("NOT IS LIST ***********")
		// }

	case "formdata":
		fmt.Println("formdata", body.FormData)
	case "file":
		fmt.Println("file")
	case "graphql":
		fmt.Println("graphql")

	}

}

// -----------------------------------------------------------------------
//
// -----------------------------------------------------------------------

func IsList(k any) bool {
	rt := reflect.TypeOf(k)
	switch rt.Kind() {
	case reflect.Slice:
		return true
	case reflect.Array:
		return true

	}
	return false
}

// -----------------------------------------------------------------------
//
// -----------------------------------------------------------------------

func IsMap(k any) bool {
	rt := reflect.TypeOf(k)

	fmt.Println(":: rt.Kind() ", rt.Kind(), reflect.TypeOf(k))
	switch rt.Kind() {
	case reflect.Map:
		return true

	}
	return false
}
