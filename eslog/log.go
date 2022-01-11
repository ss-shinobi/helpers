package eslog

import (
	"context"
	"crypto/tls"
	// "encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esutil"
	// "github.com/elastic/go-elasticsearch/v8/esapi"
	"bytes"
	"log"
	"net"
	"net/http"
	"time"

	uuid "github.com/gofrs/uuid"
	"github.com/ss-shinobi/helpers"
)

type Fields map[string]interface{}

type esLog struct {
	BulkIndexer esutil.BulkIndexer
	fields      Fields
	OnFail      func(Error)
}

type Config struct {
	EsCfg         elasticsearch.Config
	Index         string
	FlushBytes    int
	FlushInterval time.Duration
	NumWorkers    int
	OnFail        func(Error)
}

type Error struct {
	Index  string
	Reason string
}

func New(cfg Config) (esLog, error) {
	logEngine := esLog{
		OnFail: cfg.OnFail,
	}
	if cfg.EsCfg.Transport == nil {
		cfg.EsCfg.Transport = &http.Transport{
			MaxIdleConnsPerHost:   10,
			ResponseHeaderTimeout: time.Second,
			DialContext:           (&net.Dialer{Timeout: time.Second}).DialContext,
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS11,
			},
		}
	}

	es, err := elasticsearch.NewClient(cfg.EsCfg)
	if err != nil {
		return logEngine, fmt.Errorf("Error creating the client: %s", err)
	}

	logEngine.BulkIndexer, err = esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Index:         cfg.Index,         // The default index name
		Client:        es,                // The Elasticsearch client
		NumWorkers:    cfg.NumWorkers,    // The number of worker goroutines
		FlushBytes:    cfg.FlushBytes,    // The flush threshold in bytes
		FlushInterval: cfg.FlushInterval, // The periodic flush interval
	})
	if err != nil {
		return logEngine, fmt.Errorf("Error creating the Bulk Indexer: %s", err)
	}

	return logEngine, nil
}

func (e *esLog) Close() error {
	return e.BulkIndexer.Close(context.Background())
}

func (e *esLog) WithFields(fields Fields) esLog {
	return esLog{
		BulkIndexer: e.BulkIndexer,
		fields:      fields,
		OnFail:      e.OnFail,
	}
}

func (e *esLog) Debug(mss ...interface{}) error {
	log.Print(mss...)
	return e.addMess("debug", mss...)
}
func (e *esLog) Debugf(format string, mss ...interface{}) error {
	log.Printf(format, mss...)
	return e.addMess("debug", fmt.Sprintf(format, mss...))
}
func (e *esLog) Info(mss ...interface{}) error {
	log.Print(mss...)
	return e.addMess("info", mss...)
}
func (e *esLog) Infof(format string, mss ...interface{}) error {
	log.Printf(format, mss...)
	return e.addMess("info", fmt.Sprintf(format, mss...))
}
func (e *esLog) Warn(mss ...interface{}) error {
	log.Print(mss...)
	return e.addMess("warning", mss...)
}
func (e *esLog) Warnf(format string, mss ...interface{}) error {
	log.Printf(format, mss...)
	return e.addMess("warning", fmt.Sprintf(format, mss...))
}
func (e *esLog) Error(mss ...interface{}) error {
	log.Print(mss...)
	return e.addMess("error", mss...)
}
func (e *esLog) Errorf(format string, mss ...interface{}) error {
	log.Printf(format, mss...)
	return e.addMess("error", fmt.Sprintf(format, mss...))
}
func (e *esLog) Fatal(mss ...interface{}) error {
	log.Print(mss...)
	return e.addMess("fatal", mss...)
}
func (e *esLog) Fatalf(format string, mss ...interface{}) error {
	log.Printf(format, mss...)
	return e.addMess("fatal", fmt.Sprintf(format, mss...))
}

func (e *esLog) addMess(level string, mss ...interface{}) error {
	var TimeZone, _ = time.LoadLocation("UTC")
	data := Fields{
		"level":      level,
		"message":    fmt.Sprint(mss...),
		"@timestamp": time.Now().In(TimeZone).Format("2006-01-02T15:04:05"),
	}
	for k, vl := range e.fields {
		data[k] = vl
	}

	rawUuid, _ := uuid.NewV4()
	err := e.BulkIndexer.Add(
		context.Background(),
		esutil.BulkIndexerItem{
			// Action field configures the operation to perform (index, create, delete, update)
			Action:     "index",
			DocumentID: rawUuid.String(),
			Body:       bytes.NewReader(helpers.ToByte(data)),
			OnSuccess: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem) {
			},
			OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem, err error) {
				e.OnFail(Error{
					Index:  res.Index,
					Reason: res.Error.Reason,
				})
			},
		},
	)

	return err
}
