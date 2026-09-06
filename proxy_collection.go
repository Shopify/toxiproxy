func (collection *ProxyCollection) PopulateJson(
	server *ApiServer,
	data io.Reader,
) ([]*Proxy, error) {
	input := []struct {
		Proxy
		Enabled *bool `json:"enabled"` // Overrides Proxy field to make field nullable
	}{}

	err := json.NewDecoder(data).Decode(&input)
	if err != nil {
		return nil, joinError(err, ErrBadRequestBody)
	}

	// Phase 1: validate the complete request before changing collection state.
	t := true
	proxiesToApply := make([]*Proxy, 0, len(input))
	names := make(map[string]struct{}, len(input))

	for i := range input {
		if len(input[i].Name) < 1 {
			return nil, joinError(
				fmt.Errorf("name at proxy %d", i+1),
				ErrMissingField,
			)
		}

		if len(input[i].Upstream) < 1 {
			return nil, joinError(
				fmt.Errorf("upstream at proxy %d", i+1),
				ErrMissingField,
			)
		}

		if input[i].Enabled == nil {
			input[i].Enabled = &t
		}

		// Reject duplicate names inside the same populate request.
		if _, exists := names[input[i].Name]; exists {
			return nil, fmt.Errorf("duplicate proxy name %q", input[i].Name)
		}
		names[input[i].Name] = struct{}{}

		proxy := NewProxy(
			server,
			input[i].Name,
			input[i].Listen,
			input[i].Upstream,
		)

		// Enabled proxies must have valid addresses.
		// Disabled proxies are allowed to keep invalid proposed addresses.
		if *input[i].Enabled {
			if err := proxy.Validate(); err != nil {
				return nil, err
			}
		}

		proxiesToApply = append(proxiesToApply, proxy)
	}

	// Phase 2: only apply changes after every input proxy has passed
	// validation.
	proxies := make([]*Proxy, 0, len(proxiesToApply))

	for i, proxy := range proxiesToApply {
		addedOrReplaced, err := collection.AddOrReplace(
			proxy,
			*input[i].Enabled,
		)
		if err != nil {
			return nil, err
		}

		proxies = append(proxies, addedOrReplaced)
	}

	return proxies, nil
}
