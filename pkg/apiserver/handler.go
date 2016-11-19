package apiserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/emicklei/go-restful"
	"github.com/ingvagabund/cluster-capacity/cmd/cluster-capacity/app/options"
	"github.com/ingvagabund/cluster-capacity/pkg/framework"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/util/yaml"
	"strings"
	"html/template"
)

var TIMELAYOUT = "2006-01-02T15:04:05Z07:00"

type RestResource struct {
	watcher *WatchChannelDistributor
	cconf   *options.ClusterCapacityConfig
}

func (r *RestResource) Register(container *restful.Container) {
	ws := new(restful.WebService)
	ws.
		Path("/").
		Doc("Manage cluster capacity checker").
		Consumes(restful.MIME_XML, restful.MIME_JSON).
		Produces(restful.MIME_JSON, restful.MIME_XML)

	ws.Route(ws.GET("/").To(r.introduce))

	ws.Route(ws.GET("/capacity").To(r.introduce))

	ws.Route(ws.GET("/capacity/pod").To(r.getPod).
		Doc("Get pod used for counting cluster capacity").
		Operation("getPod").
		Param(ws.QueryParameter("pretty", "pretty print pod").DataType("boolean")).
		Writes(restful.MIME_JSON))

	ws.Route(ws.PUT("/capacity/pod").To(r.putPod).
		Doc("Update pod used for counting cluster capacity").
		Operation("putPod"))

	ws.Route(ws.GET("/capacity/status").To(r.getLastStatus).
		Doc("Get most recent cluster capacity report").
		Param(ws.QueryParameter("num", "number of last records to be listed").DataType("string")).
		Param(ws.QueryParameter("since", "RFC3339 standard").DataType("string")).
		Param(ws.QueryParameter("to", "RFC3339 standard").DataType("string")).
		Param(ws.QueryParameter("watch", "get notification for new ones").DataType("boolean")).
		Operation("getStatus").
		Writes([]framework.Report{}))
	container.Add(ws)
}

func NewResource(watch chan *framework.Report, conf *options.ClusterCapacityConfig) *RestResource {
	return &RestResource{
		watcher: NewWatchChannelDistributor(watch),
		cconf:   conf,
	}
}

func ListenAndServe(r *RestResource) error {
	wsContainer := restful.NewContainer()
	r.Register(wsContainer)

	go r.watcher.Run()
	server := &http.Server{Addr: ":8081", Handler: wsContainer}
	return server.ListenAndServe()
}

type ccBasicInfo struct {
	CacheSize int
	Period    int
}

func (r *RestResource) introduce(request *restful.Request, response *restful.Response) {
	response.AddHeader("Content-Type", "text/html")

	info := &ccBasicInfo{
		CacheSize: r.cconf.Reports.GetSize(),
		Period:    r.cconf.Options.Period,
	}
	t, err := template.ParseFiles("/doc/html/home.html")
	if err != nil {
		fmt.Printf("Template gave: %s", err)
	}
	t.Execute(response.ResponseWriter, info)
}

func (r *RestResource) getLastStatus(request *restful.Request, response *restful.Response) {
	// parse parameters
	numStr := request.QueryParameter("num")
	if numStr == "" {
		numStr = "-1"
	}
	num, err := strconv.Atoi(numStr)

	watch := request.QueryParameter("watch")

	//if since is not defined set is as start of epoch
	sinceStr := request.QueryParameter("since")
	if sinceStr == "" {
		sinceStr = "1970-01-01T00:00:00+00:00"
	}
	since, err := time.Parse(TIMELAYOUT, sinceStr)
	if err != nil {
		str := fmt.Sprintf("400: Failed to parse parameter \"since\": %v\n", err)
		response.WriteErrorString(http.StatusBadRequest, str)
		return
	}

	//if to is not defined set it as now
	toStr := request.QueryParameter("to")
	var to time.Time
	if toStr == "" {
		to = time.Now()
	} else {
		to, err = time.Parse(TIMELAYOUT, toStr)
		if err != nil {
			str := fmt.Sprintf("400: Failed to parse parameter \"to\": %v\n", err)
			response.WriteErrorString(http.StatusBadRequest, str)
			return
		}
	}

	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		str := fmt.Sprintf("400: Failed to parse parameter \"num\": %v\n", err)
		response.WriteErrorString(http.StatusBadRequest, str)
		return
	}

	report := r.cconf.Reports.List(since, to, num)
	if report == nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusNotFound, "404: No reports found.")
		return
	}
	response.WriteAsJson(report)
	if watch == "true" {
		r.watchStatus(request, response)
	}
}

// use this to avoid multiple response.WriteHeader calls
func writeJson(resp *restful.Response, r *framework.Report) error {
	output, err := json.MarshalIndent(r, " ", " ")
	if err != nil {
		return err
	}
	_, err = resp.Write(output)
	return err
}

func (r *RestResource) watchStatus(request *restful.Request, response *restful.Response) {
	w := response.ResponseWriter

	//receive read channel
	ch := make(chan *framework.Report)
	chpos, err := r.watcher.AddChannel(ch)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		errmsg := fmt.Sprintf("Can't start watching: %v", err)
		response.WriteErrorString(http.StatusForbidden, errmsg)
		return
	}
	defer r.watcher.RemoveChannel(chpos)


	w.Header().Set("Transfer-Encoding", "chunked")
	w.(http.Flusher).Flush()

	//listen read channel
	for {
		select {
		case <-w.(http.CloseNotifier).CloseNotify():
			return
		case report, ok := <-ch:
			if !ok {
				return
			}
			if err := writeJson(response, report); err != nil {
				continue

			}
			w.(http.Flusher).Flush()
		}
	}
}

func (r *RestResource) getPod(request *restful.Request, response *restful.Response) {
	if request.QueryParameter("pretty") == "true" {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, "Pretty print not implemented yet\n")
	}
	response.WriteAsJson(r.cconf.Pod)
}

func (r *RestResource) putPod(request *restful.Request, response *restful.Response) {
	if r.cconf.Pod != nil {
		r.cconf.Pod = &api.Pod{}
	}
	decoder := yaml.NewYAMLOrJSONDecoder(request.Request.Body, 4096)
	err := decoder.Decode(r.cconf.Pod)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		msg := fmt.Sprintf("Failed to decode pod: %v\n", err)
		response.WriteErrorString(http.StatusInternalServerError, msg)
		return
	}

	if errs := validation.ValidatePod(r.cconf.Pod); len(errs) > 0 {
		var errStrs []string
		for _, err := range errs {
			errStrs = append(errStrs, fmt.Sprintf("%v: %v", err.Type, err.Field))
		}
		msg := fmt.Sprintf("Invalid pod: %#v", strings.Join(errStrs, ", "))
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, msg)
		return
	}

	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}
	response.WriteHeaderAndEntity(http.StatusCreated, r.cconf.Pod)
}
