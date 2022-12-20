package srv

import (
	"encoding/json"
	"net/http"

	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"

	"go.infratographer.com/x/pubsubx"

	events "go.infratographer.com/loadbalanceroperator/pkg/events/v1alpha1"
)

type valueSet struct {
	helmKey string
	value   string
}

// MessageHandler handles the routing of events from specified queues
func (s *Server) MessageHandler(m *nats.Msg) {
	msg := pubsubx.Message{}
	if err := json.Unmarshal(m.Data, &msg); err != nil {
		s.Logger.Errorw("Unable to process data in message: %s", "error", err)
	}

	switch msg.EventType {
	case events.EVENTCREATE:
		if err := s.createMessageHandler(&msg); err != nil {
			s.Logger.Errorw("unable to process create: %s", "error", err)
		}
	case events.EVENTUPDATE:
		err := s.updateMessageHandler(&msg)
		if err != nil {
			s.Logger.Errorw("unable to process update", "error", err.Error())
		}
	default:
		s.Logger.Debug("This is some other set of queues that we don't know about.")
	}
}

func (s *Server) createMessageHandler(m *pubsubx.Message) error {
	lbdata := events.LoadBalancerData{}

	if err := s.parseLBData(&m.AdditionalData, &lbdata); err != nil {
		s.Logger.Errorw("handler unable to parse loadbalancer data", "error", err)
		return err
	}

	if err := s.CreateNamespace(m.SubjectURN); err != nil {
		s.Logger.Errorw("handler unable to create required namespace", "error", err)
		return err
	}

	overrides := []valueSet{}
	for _, cpuFlag := range viper.GetStringSlice("helm-cpu-flag") {
		overrides = append(overrides, valueSet{
			helmKey: cpuFlag,
			value:   lbdata.Resources.CPU,
		})
	}

	for _, memFlag := range viper.GetStringSlice("helm-memory-flag") {
		overrides = append(overrides, valueSet{
			helmKey: memFlag,
			value:   lbdata.Resources.Memory,
		})
	}

	if err := s.CreateApp(lbdata.LoadBalancerID.String(), m.SubjectURN, overrides); err != nil {
		s.Logger.Errorw("handler unable to create loadbalancer", "error", err)
		return err
	}

	return nil
}

func (s *Server) updateMessageHandler(m *pubsubx.Message) error {
	return nil
}

// ExposeEndpoint exposes a specified port for various checks
func (s *Server) ExposeEndpoint(subscription *nats.Subscription, port string) error {
	if port == "" {
		return ErrPortsRequired
	}

	go func() {
		s.Logger.Infof("Starting endpoints on %s", port)

		checkConfig := http.NewServeMux()
		checkConfig.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("ok"))
		})
		checkConfig.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
			if !subscription.IsValid() {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("500 - Queue subscription is inactive"))
			} else {
				_, _ = w.Write([]byte("ok"))
			}
		})

		checks := http.Server{
			Handler: checkConfig,
			Addr:    port,
		}

		_ = checks.ListenAndServe()
	}()

	return nil
}

func (s *Server) parseLBData(data *map[string]interface{}, lbdata *events.LoadBalancerData) error {
	d, err := json.Marshal(data)
	if err != nil {
		s.Logger.Errorw("unable to load data from event", "error", err.Error())
		return err
	}

	if err := json.Unmarshal(d, &lbdata); err != nil {
		s.Logger.Errorw("unable to parse event data", "error", err.Error())
		return err
	}

	return nil
}
