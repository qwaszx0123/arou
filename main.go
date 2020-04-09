package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strconv"

	"github.com/pborman/uuid"
	elastic "gopkg.in/olivere/elastic.v3"
)

type Location struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// {
// 	“user”: “jack”
// 	“message”: “this is a message”
// 	"localtion": {
// 		"lat:34
// 		"lan:1234
// 	}
//   }
//

//由于go 首字母大写代表 pulix 和json默认不一样 所以要用 json:"user"来建立连接

type Post struct {
	// `json:"user"` is for the json parsing of this User field. Otherwise, by default it's 'User'.
	User     string   `json:"user"`
	Message  string   `json:"message"`
	Location Location `json:"location"`
}

const (
	INDEX    = "around"
	TYPE     = "post"
	DISTANCE = "200km"
	// Needs to update
	//PROJECT_ID = "around-xxx"
	//BT_INSTANCE = "around-post"
	// Needs to update this URL if you deploy it to cloud.
	ES_URL = "http://34.71.174.48:9200"
)

//这里没有说是 post还是 get 那么都可以 但是代码里我们只是编写post
func handlerSearch(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received one request for search")
	// you两个返回值 下划线表示 不care error
	lat, _ := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lon, _ := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
	// range is optional
	ran := DISTANCE
	if val := r.URL.Query().Get("range"); val != "" {
		ran = val + "km"
	}
	fmt.Printf("Search received: %f %f %s\n", lat, lon, ran)

	// Create a client handler  什么叫sniff  我们每次操作的时候 可以有回调函数 现在不需要 所以false    elastic
	//是服务  现在式用api 操作服务     拿到了error 就要判断一下
	client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false))
	if err != nil {
		panic(err)
		return
	}

	// Define geo distance query as specified in  有了client之后要进行搜索
	// https://www.elastic.co/guide/en/elasticsearch/reference/5.2/query-dsl-geo-distance-query.html
	q := elastic.NewGeoDistanceQuery("location")
	//q要设置参数  一下相当于chain 一块的调用 每次都是返回q
	q = q.Distance(ran).Lat(lat).Lon(lon)

	//现在开始搜索了 结果在result    pretty ture的话是返回json    之前都是设置参数  do才是执行
	//client 是与 elastic search链接的 工具

	// Some delay may range from seconds to minutes. So if you don't get enough results. Try it later.
	searchResult, err := client.Search().
		Index(INDEX).
		Query(q).
		Pretty(true).
		Do()
	if err != nil {
		// Handle error
		panic(err)
	}

	// searchResult is of type SearchResult and returns hits, suggestions,
	// and all kinds of other information from Elasticsearch.
	fmt.Printf("Query took %d milliseconds\n", searchResult.TookInMillis)
	// TotalHits is another convenience function that works even when something goes wrong.
	fmt.Printf("Found a total of %d post\n", searchResult.TotalHits())

	// Each is a convenience function that iterates over hits in a search result.
	// It makes sure you don't need to check for nil values in the response.
	// However, it ignores errors in serialization.
	var typ Post
	var ps []Post
	//这里 relection 只从结果里取得 类型是post的东西 相当于instance of
	for _, item := range searchResult.Each(reflect.TypeOf(typ)) { // instance of
		p := item.(Post) // p = (Post) item
		fmt.Printf("Post by %s: %s at lat %v and lon %v\n", p.User, p.Message, p.Location.Lat, p.Location.Lon)
		// TODO(student homework): Perform filtering based on keywords such as web spam etc.
		ps = append(ps, p)

	}
	js, err := json.Marshal(ps)
	if err != nil {
		panic(err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(js)

}

// func handlerSearch(w http.ResponseWriter, r *http.Request) {
// 	fmt.Println("Received one request for search")
// 	lat := r.URL.Query().Get("lat")
// 	lon := r.URL.Query().Get("lon")

// 	fmt.Fprintf(w, "Search received: %s %s", lat, lon)
// }

//这里xin  号是指针 避免拷贝  go参数所有都是按值传递 , 传进来后 类似primitive int
func handlerPost(w http.ResponseWriter, r *http.Request) {
	// Parse from body of request to get a json object.
	fmt.Println("Received one post request")
	decoder := json.NewDecoder(r.Body)
	var p Post
	//panic 相当于throw ; 类似换行 两个语句写一行
	//if 写两个statment 第一statment 初始化 第二个statment 做具体判断,
	//decode 里面接受的是指针参数 所以用&
	if err := decoder.Decode(&p); err != nil {
		panic(err)
		return
	}

	fmt.Fprintf(w, "Post received: %s\n", p.Message)
	id := uuid.New()
	// Save to ES.
	saveToES(&p, id)

}

// Save a post to ElasticSearch
func saveToES(p *Post, id string) {
	// Create a client
	es_client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false))
	if err != nil {
		panic(err)
		return
	}

	// Save it to index  如果有重复新的代替旧的 refresh   uuid 表示自动生成几乎 unique的string key
	_, err = es_client.Index().
		Index(INDEX).
		Type(TYPE).
		Id(id).
		BodyJson(p).
		Refresh(true).
		Do()
	if err != nil {
		panic(err)
		return
	}

	fmt.Printf("Post is saved to Index: %s\n", p.Message)
}
func main() {
	// Create a client
	client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false))
	if err != nil {
		panic(err)
		return
	}

	// Use the IndexExists service to check if a specified index exists.
	exists, err := client.IndexExists(INDEX).Do()
	if err != nil {
		panic(err)
	}
	if !exists {
		// Create a new index. 把location 标记成 geopoint
		mapping := `{
			"mappings":{
				"post":{
					"properties":{
						"location":{
							"type":"geo_point"
						}
					}
				}
			}
		}`
		_, err := client.CreateIndex(INDEX).Body(mapping).Do()
		if err != nil {
			// Handle error
			panic(err)
		}
	}

	fmt.Println("started-service")
	http.HandleFunc("/post", handlerPost)
	http.HandleFunc("/search", handlerSearch)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
