package main

import (
	"fmt"
	"net/http"
)

type req struct {
	Cloud   string //云商
	SoloID  string //solo id
	CmdbID  string //有需要可以去CMDB查询
	CloudID string //云商ID
	Address string //地址
}

func (r *req) String() string {
	return r.Cloud + ":" + r.SoloID + ":" + r.CmdbID + ":" + r.CloudID + ":" + r.Address
}

func banding(r *http.Request) (*req, error) {
	values := r.URL.Query()
	var request req
	if address := values.Get("address"); len(address) != 0 {
		request.Address = address
	} else {
		return nil, fmt.Errorf("Request parameter error  %s  ", "address")
	}
	if cmdbID := values.Get("cmdb_id"); len(cmdbID) != 0 {
		request.CmdbID = cmdbID
	} else {
		return nil, fmt.Errorf("Request parameter error  %s  ", "cmdb_id")
	}
	if cloudID := values.Get("cloud_id"); len(cloudID) != 0 {
		request.CloudID = cloudID
	} else {
		return nil, fmt.Errorf("Request parameter error  %s  ", "cloud_id")
	}
	if cloud := values.Get("cloud"); len(cloud) != 0 {
		request.Cloud = cloud
	} else {
		return nil, fmt.Errorf("Request parameter error  %s  ", "cloud")
	}
	if soloID := values.Get("solo_id"); len(soloID) != 0 {
		request.SoloID = soloID
	} else {
		return nil, fmt.Errorf("Request parameter error  %s  ", "solo_id")
	}
	return &request, nil
}
