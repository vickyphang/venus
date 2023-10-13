package gin_logger

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	gologging "github.com/devopsfaith/krakend-gologging"
	logstash "github.com/devopsfaith/krakend-logstash"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/luraproject/lura/config"
	"github.com/luraproject/lura/logging"
)

const (
	Namespace  = "github_com/vickyphang/venus"
	moduleName = "venus"
)

var host, user, password, dbname string
var port int

func NewLogger(cfg config.ExtraConfig, logger logging.Logger, loggerConfig gin.LoggerConfig) gin.HandlerFunc {
	v, ok := ConfigGetter(cfg).(Config)
	if !ok {
		return gin.LoggerWithConfig(loggerConfig)
	}

	host = v.Host
	port = v.Port
	user = v.User
	password = v.Pass
	dbname = v.DBname

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

	// For testing purpose
	//fmt.Println(host, port, user, password, dbname)

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
	_, err = db.Exec("INSERT INTO "+dbname+" (timestamp, client_id, method, status, path, details) VALUES($1, $2, $3, $4, $5, $6)", timestamp, client_token, method, status, path, data)
	if err != nil {
		panic(err)
	}

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

	cfg := Config{}
	if skipPaths, ok := tmp["skip_paths"].([]interface{}); ok {
		var paths []string
		for _, skipPath := range skipPaths {
			if path, ok := skipPath.(string); ok {
				paths = append(paths, path)
			}
		}
		cfg.SkipPaths = paths
	}
	cfg.Logstash = false
	if v, ok = e[gologging.Namespace]; ok {
		_, cfg.Logstash = e[logstash.Namespace]
	}

	if v, ok := tmp["host"].(string); ok {
		cfg.Host = v
	}
	if v, ok := tmp["port"]; ok {
		cfg.Port = int(v.(float64))
	}
	if v, ok := tmp["user"].(string); ok {
		cfg.User = v
	}
	if v, ok := tmp["pass"].(string); ok {
		cfg.Pass = v
	}
	if v, ok := tmp["dbname"].(string); ok {
		cfg.DBname = v
	}

	return cfg
}

type Config struct {
	SkipPaths []string
	Logstash  bool
	Host      string
	Port      int
	User      string
	Pass      string
	DBname    string
}
