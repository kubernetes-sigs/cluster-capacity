package apiserver

import (
	"github.com/emicklei/go-restful"
	"net/http"
)

type RestResource struct {
	cache *Cache
}

//TODO: add posibility to list records in time interval
//TODO: handle watch method
func (r *RestResource) Register(container *restful.Container) {
	ws := new(restful.WebService)
	ws.
		Path("/capacity").
		Doc("Manage cluster capacity checker").
		Consumes(restful.MIME_XML, restful.MIME_JSON).
		Produces(restful.MIME_JSON, restful.MIME_XML)

	ws.Route(ws.GET("/status/last").To(r.getStatus).
		Doc("Get most recent cluster capacity report").
		Operation("getStatus").
		Writes(Report{}))

	ws.Route(ws.GET("/status/list/").To(r.listStatus).
		Doc("List cluster capacity reports").
		Operation("listStatus").
		Writes([]Report{}))

	container.Add(ws)
}

func NewResource(c *Cache) *RestResource {
	return &RestResource{
		cache: c,
	}
}
func ListenAndServe(r *RestResource) error {
	wsContainer := restful.NewContainer()
	r.Register(wsContainer)

	server := &http.Server{Addr: ":8081", Handler: wsContainer}
	return server.ListenAndServe()
}

func (r *RestResource) getStatus(request *restful.Request, response *restful.Response) {
	report := r.cache.GetLast()
	if report == nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusNotFound, "404: No reports found.")
		return
	}
	response.WriteAsJson(report)
}

func (r *RestResource) listStatus(request *restful.Request, response *restful.Response) {
	reports := r.cache.All()
	if reports == nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusNotFound, "404: No reports found.")
		return
	}
	response.WriteAsJson(reports)
}
