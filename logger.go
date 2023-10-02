// Copyright 2021 Faisal Alam. All rights reserved.
// Use of this source code is governed by a Apache style
// license that can be found in the LICENSE file.

package gin_logger

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"

	//	"log"
	"io/ioutil"
	//"os"
	gologging "github.com/devopsfaith/krakend-gologging"
	logstash "github.com/devopsfaith/krakend-logstash"
	"github.com/gin-gonic/gin"
	"github.com/luraproject/lura/config"
	"github.com/luraproject/lura/logging"
	//	logstash "github.com/krakendio/krakend-logstash/v2"
	//	"github.com/krakendio/krakend-gologging/v2"
	//	"github.com/luraproject/lura/v2/config"
	//	"github.com/luraproject/lura/v2/logging"
)

const (
	Namespace  = "github_com/vickyphang/venus"
	moduleName = "venus"
)

var host, user, password, dbname string
var port int64

func NewLogger(cfg config.ExtraConfig, logger logging.Logger, loggerConfig gin.LoggerConfig) gin.HandlerFunc {

	f, ok := DatabaseGetter(cfg).(Database)
	if !ok {
		panic(ok)
	}
	host = f.Host
	port = f.Port
	user = f.Username
	password = f.Password
	dbname = f.DBname
	strings.Trim(password, "\"")
	strings.Trim(user, "\"")
	strings.Trim(host, "\"")
	strings.Trim(dbname, "\"")

	v, ok := ConfigGetter(cfg).(Config)
	if !ok {
		return gin.LoggerWithConfig(loggerConfig)
	}

	loggerConfig.SkipPaths = v.SkipPaths
	logger.Info(fmt.Sprintf("%s: total skip paths set: %d", moduleName, len(v.SkipPaths)))

	loggerConfig.Output = ioutil.Discard
	loggerConfig.Formatter = Formatter{logger, v}.DefaultFormatter
	return gin.LoggerWithConfig(loggerConfig)
}

type Formatter struct {
	logger logging.Logger
	config Config
}

func (f Formatter) DefaultFormatter(params gin.LogFormatterParams) string {
	header := params.Request.Header
	body := params.Request.Body
	method := params.Method
	path := params.Path
	status := params.StatusCode

	//	reqBody, _ := io.ioutil.ReadAll(body)
	//	reqHeader, _ := io.ioutil.ReadAll(header)
	record := map[string]interface{}{
		"method":             method,
		"host":               params.Request.Host,
		"path":               path,
		"status_code":        status,
		"user_agent":         params.Request.UserAgent(),
		"client_ip":          params.ClientIP,
		"latency":            params.Latency,
		"response_timestamp": params.TimeStamp,
		"body":               body,
		"header":             header,
	}

	payload := map[string]interface{}{
		"data": record,
	}

	//	a, _ := json.Marshal(payload)
	//	file, err := os.OpenFile("/home/ubuntu/gin.log", os.O_CREATE|os.O_WRONLY, 0644)
	// 	if err != nil {
	//		log.Fatal(err)
	//	}
	//	defer file.Close()
	//	_, err = file.WriteString(string(a))
	//
	//	if err != nil {
	//		log.Fatal(err)
	//	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	timestamp := time.Now().In(loc)
	client_token := ""
	if header["Authorization"] != nil {
		client_tokenAddr := &client_token
		*client_tokenAddr = strings.ReplaceAll(fmt.Sprint(header["Authorization"]), "Bearer ", "")
	}

	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+"password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	data, _ := json.Marshal(payload)
	_, err = db.Exec("INSERT INTO krakend (timestamp, client_id, method, status, path, details) VALUES($1, $2, $3, $4, $5, $6)", timestamp, client_token, method, status, path, data)
	if err != nil {
		panic(err)
	}
	//	id := 0
	//	sqlStatement := `INSERT INTO items (data) VALUES ($1)`
	//	err = db.QueryRow(sqlStatement, string(a)).Scan(&id)
	//	if err != nil {
	//		panic(err)
	//	}

	if f.config.Logstash {
		f.logger.Info("", payload)
	} else {
		p, _ := json.Marshal(payload)
		f.logger.Info(string(p))
	}

	return ""
}

func ConfigGetter(e config.ExtraConfig) interface{} {
	v, ok := e[Namespace]
	if !ok {
		return nil
	}
	tmp, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}

	cfg := defaultConfigGetter()
	if skipPaths, ok := tmp["skip_paths"].([]interface{}); ok {
		var paths []string
		for _, skipPath := range skipPaths {
			if path, ok := skipPath.(string); ok {
				paths = append(paths, path)
			}
		}
		cfg.SkipPaths = paths
	}
	if v, ok = e[gologging.Namespace]; ok {
		_, cfg.Logstash = e[logstash.Namespace]
	}

	return cfg
}

func DatabaseGetter(e config.ExtraConfig) interface{} {
	v, ok := e[Namespace]
	if !ok {
		return nil
	}
	tmp, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}

	database := defaultDatabaseGetter()
	host, _ := tmp["host"].(string)
	port, _ := tmp["port"].(string)
	user, _ := tmp["user"].(string)
	pass, _ := tmp["password"].(string)
	dbname, _ := tmp["dbname"].(string)

	database.Host = host
	database.Host = port
	database.Username = user
	database.Password = pass
	database.DBname = dbname

	return database
}

func defaultConfigGetter() Config {
	return Config{
		SkipPaths: []string{},
		Logstash:  false,
	}
}

func defaultDatabaseGetter() Database {
	return Database{
		Host:     "localhost",
		Port:     5432,
		Username: "user",
		Password: "pass",
		DBname:   "user",
	}
}

type Config struct {
	SkipPaths []string
	Logstash  bool
}

type Database struct {
	Port     int64
	Host     string
	Username string
	Password string
	DBname   string
}
