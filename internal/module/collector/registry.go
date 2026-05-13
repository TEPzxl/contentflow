package collector

import "fmt"

type Registry struct {
	collectors map[string]Collector
}

func NewRegistry(collectors ...Collector) (*Registry, error) {
	r := &Registry{
		collectors: make(map[string]Collector),
	}

	for _, c := range collectors {
		if c == nil {
			continue
		}

		sourceType := c.Type()
		if sourceType == "" {
			return nil, fmt.Errorf("collector type is empty")
		}

		if _, exists := r.collectors[sourceType]; exists {
			return nil, fmt.Errorf("collector duplicated: %s", sourceType)
		}

		r.collectors[sourceType] = c
	}

	return r, nil
}

func (r *Registry) Get(sourceType string) (Collector, bool) {
	c, exists := r.collectors[sourceType]
	return c, exists
}
