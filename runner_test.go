package main

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/hashicorp/consul-template/config"
	"github.com/hashicorp/consul-template/dependency"
)

func TestRunner_appendSecrets(t *testing.T) {
	t.Parallel()

	secrets := []string{"somevalue1", "somevalue2"}

	tt := []struct {
		name     string
		path     string
		noPrefix *bool
		data     *dependency.Secret
		keyNames []string
		notFound bool
	}{
		{
			name:     "kv1 secret",
			path:     "kv/foo",
			noPrefix: config.Bool(false),
			data: &dependency.Secret{
				Data: map[string]interface{}{
					"bar": secrets[0],
					"zed": secrets[1],
				},
			},
			keyNames: []string{"kv_foo_bar", "kv_foo_zed"},
			notFound: false,
		},
		{
			name:     "kv2 secret",
			path:     "secret/data/foo",
			noPrefix: config.Bool(false),
			data: &dependency.Secret{
				Data: map[string]interface{}{
					"metadata": map[string]interface{}{
						"destroyed": bool(false),
						"version":   "1",
					},
					"data": map[string]interface{}{
						"bar": secrets[0],
						"zed": secrets[1],
					},
				},
			},
			keyNames: []string{"secret_data_foo_bar", "secret_data_foo_zed"},
			notFound: false,
		},
		{
			name:     "kv2 secret destroyed",
			path:     "secret/data/foo",
			noPrefix: config.Bool(false),
			data: &dependency.Secret{
				Data: map[string]interface{}{
					"metadata": map[string]interface{}{
						"destroyed": bool(true),
						"version":   "2",
					},
					"data": nil,
				},
			},
			keyNames: []string{},
			notFound: true,
		},
		{
			name:     "kv2 secret noprefix excludes path",
			path:     "secret/data/foo",
			noPrefix: config.Bool(true),
			data: &dependency.Secret{
				Data: map[string]interface{}{
					"metadata": map[string]interface{}{
						"destroyed": bool(false),
						"version":   "1",
					},
					"data": map[string]interface{}{
						"bar": secrets[0],
						"zed": secrets[1],
					},
				},
			},
			keyNames: []string{"bar", "zed"},
			notFound: false,
		},
		{
			name:     "kv2 secret false noprefix includes path",
			path:     "secret/data/foo",
			noPrefix: config.Bool(false),
			data: &dependency.Secret{
				Data: map[string]interface{}{
					"metadata": map[string]interface{}{
						"destroyed": bool(false),
						"version":   "1",
					},
					"data": map[string]interface{}{
						"bar": secrets[0],
						"zed": secrets[1],
					},
				},
			},
			keyNames: []string{"secret_data_foo_bar", "secret_data_foo_zed"},
			notFound: false,
		},
		{
			name:     "kv2 secret default noprefix includes path",
			path:     "secret/data/foo",
			noPrefix: nil,
			data: &dependency.Secret{
				Data: map[string]interface{}{
					"metadata": map[string]interface{}{
						"destroyed": bool(false),
						"version":   "1",
					},
					"data": map[string]interface{}{
						"bar": secrets[0],
						"zed": secrets[1],
					},
				},
			},
			keyNames: []string{"secret_data_foo_bar", "secret_data_foo_zed"},
			notFound: false,
		},
		{
			name:     "int secret skipped",
			path:     "kv/foo",
			noPrefix: config.Bool(false),
			data: &dependency.Secret{
				Data: map[string]interface{}{
					"bar": 1,
					"zed": 1,
				},
			},
			notFound: true,
		},
	}

	for _, tc := range tt {
		t.Run(fmt.Sprintf("%s", tc.name), func(t *testing.T) {
			cfg := Config{
				Secrets: &PrefixConfigs{
					&PrefixConfig{
						Path:     config.String(tc.path),
						NoPrefix: tc.noPrefix,
					},
				},
			}
			c := DefaultConfig().Merge(&cfg)
			r, err := NewRunner(c, true)
			if err != nil {
				t.Fatal(err)
			}
			vrq, err := dependency.NewVaultReadQuery(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			env := make(map[string]string)
			appendError := r.appendSecrets(env, vrq, tc.data)
			if appendError != nil {
				t.Fatalf("got err: %s", appendError)
			}

			if len(env) > 2 {
				t.Fatalf("Expected only 2 values in this test")
			}

			for i, keyName := range tc.keyNames {
				secretValue := secrets[i]

				var value string
				value, ok := env[keyName]
				if !ok && !tc.notFound {
					t.Fatalf("expected (%s) key, but was not found", keyName)
				}
				if ok && tc.notFound {
					t.Fatalf("expected to not find key, but (%s) was found",
						keyName)
				}
				if ok && value != secretValue {
					t.Fatalf("values didn't match, expected (%s), got (%s)",
						secretValue, value)
				}
			}
		})
	}
}

func TestRunner_appendPrefixes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		path     string
		noPrefix *bool
		data     []*dependency.KeyPair
		keyName  string
	}{
		{
			name:     "false noprefix appends path",
			path:     "app/my_service",
			noPrefix: config.Bool(false),
			data: []*dependency.KeyPair{
				&dependency.KeyPair{
					Key:   "mykey",
					Value: "myValue",
				},
			},
			keyName: "app_my_service_mykey",
		},
		{
			name:     "true noprefix excludes path",
			path:     "app/my_service",
			noPrefix: config.Bool(true),
			data: []*dependency.KeyPair{
				&dependency.KeyPair{
					Key:   "mykey",
					Value: "myValue",
				},
			},
			keyName: "mykey",
		},
		{
			name:     "null noprefix excludes path",
			path:     "app/my_service",
			noPrefix: nil,
			data: []*dependency.KeyPair{
				&dependency.KeyPair{
					Key:   "mykey",
					Value: "myValue",
				},
			},
			keyName: "mykey",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{
				Prefixes: &PrefixConfigs{
					&PrefixConfig{
						Path:     config.String(tc.path),
						NoPrefix: tc.noPrefix,
					},
				},
			}
			c := DefaultConfig().Merge(&cfg)
			r, err := NewRunner(c, true)
			if err != nil {
				t.Fatal(err)
			}
			kvq, err := dependency.NewKVListQuery(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			env := make(map[string]string)
			appendError := r.appendPrefixes(env, kvq, tc.data)
			if appendError != nil {
				t.Fatalf("got err: %s", appendError)
			}

			if len(env) > 1 {
				t.Fatalf("Expected only 1 value in this test")
			}

			var value string
			value, ok := env[tc.keyName]
			if !ok {
				t.Fatalf("expected (%s) key, but was not found", tc.keyName)
			}
			if ok && value != tc.data[0].Value {
				t.Fatalf("values didn't match, expected (%s), got (%s)", tc.data[0].Value, value)
			}
		})
	}
}

func TestRunner_appendServices(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		query          string
		config         Config
		data           []*dependency.CatalogService
		keyValue       map[string]string
		serviceID      string
		serviceName    string
		serviceAddress string
		serviceTag     string
		servicePort    string
	}{
		{
			name:   "service appends data",
			query:  "service",
			config: Config{},
			data: []*dependency.CatalogService{
				&dependency.CatalogService{
					ServiceID:      "id",
					ServiceName:    "foo",
					ServiceAddress: "address",
					ServiceTags:    dependency.ServiceTags{"tag1", "tag2"},
					ServicePort:    8080,
				},
			},
			keyValue: map[string]string{
				"foo/id":      "id",
				"foo/name":    "foo",
				"foo/address": "address",
				"foo/tag":     "tag1,tag2",
				"foo/port":    "8080",
			},
			serviceID:      "foo/id",
			serviceName:    "foo/name",
			serviceAddress: "foo/address",
			serviceTag:     "foo/tag",
			servicePort:    "foo/port",
		},
		{
			name:   "service appends data",
			query:  "service",
			config: Config{},
			data: []*dependency.CatalogService{
				&dependency.CatalogService{
					ServiceID:      "fail_id",
					ServiceName:    "foo",
					ServiceAddress: "fail_address",
					ServiceTags:    dependency.ServiceTags{"tag1"},
					ServicePort:    8081,
				},
				&dependency.CatalogService{
					ServiceID:      "id",
					ServiceName:    "foo",
					ServiceAddress: "address",
					ServiceTags:    dependency.ServiceTags{"tag1", "tag2"},
					ServicePort:    8080,
				},
			},
			keyValue: map[string]string{
				"foo/id":      "id",
				"foo/name":    "foo",
				"foo/address": "address",
				"foo/tag":     "tag1,tag2",
				"foo/port":    "8080",
			},
			serviceID:      "foo/id",
			serviceName:    "foo/name",
			serviceAddress: "foo/address",
			serviceTag:     "foo/tag",
			servicePort:    "foo/port",
		},
		{
			name:  "service appends data with a custom format",
			query: "service",
			config: Config{
				Services: &ServiceConfigs{
					&ServiceConfig{
						Query:         config.String("service"),
						FormatId:      config.String("{{key}}/{{service}}/test"),
						FormatName:    config.String("{{key}}/{{service}}/test"),
						FormatAddress: config.String("{{key}}/{{service}}/test"),
						FormatTag:     config.String("{{key}}/{{service}}/test"),
						FormatPort:    config.String("{{key}}/{{service}}/test"),
					},
				},
			},
			data: []*dependency.CatalogService{
				&dependency.CatalogService{
					ServiceID:      "id",
					ServiceName:    "foo",
					ServiceAddress: "address",
					ServiceTags:    dependency.ServiceTags{"tag1", "tag2"},
					ServicePort:    8080,
				},
			},
			keyValue: map[string]string{
				"id/foo/test":      "id",
				"name/foo/test":    "foo",
				"address/foo/test": "address",
				"tag/foo/test":     "tag1,tag2",
				"port/foo/test":    "8080",
			},
			serviceID:      "id/foo/test",
			serviceName:    "name/foo/test",
			serviceAddress: "address/foo/test",
			serviceTag:     "tag/foo/test",
			servicePort:    "port/foo/test",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := DefaultConfig().Merge(&tc.config)
			r, err := NewRunner(c, true)
			if err != nil {
				t.Fatal(err)
			}

			csq, err := dependency.NewCatalogServiceQuery(tc.query)
			if err != nil {
				t.Fatal(err)
			}
			env := make(map[string]string)
			appendError := r.appendServices(env, csq, tc.data)
			if appendError != nil {
				t.Fatalf("got err: %s", appendError)
			}

			if len(env) != 5 {
				t.Fatalf("Expected only 1 value in this test")
			}

			value, ok := env[tc.serviceID]
			if !ok {
				t.Fatalf("expected (%s) key, but was not found", tc.serviceID)
			}
			if ok && value != tc.keyValue[tc.serviceID] {
				t.Fatalf("values didn't match, expected (%s), got (%s)", tc.keyValue[tc.serviceID], value)
			}

			value, ok = env[tc.serviceName]
			if !ok {
				t.Fatalf("expected (%s) key, but was not found", tc.serviceName)
			}
			if ok && value != tc.keyValue[tc.serviceName] {
				t.Fatalf("values didn't match, expected (%s), got (%s)", tc.keyValue[tc.serviceName], value)
			}

			value, ok = env[tc.serviceAddress]
			if !ok {
				t.Fatalf("expected (%s) key, but was not found", tc.serviceAddress)
			}
			if ok && value != tc.keyValue[tc.serviceAddress] {
				t.Fatalf("values didn't match, expected (%s), got (%s)", tc.keyValue[tc.serviceAddress], value)
			}

			value, ok = env[tc.serviceTag]
			if !ok {
				t.Fatalf("expected (%s) key, but was not found", tc.serviceTag)
			}
			if ok && value != tc.keyValue[tc.serviceTag] {
				t.Fatalf("values didn't match, expected (%s), got (%s)", tc.keyValue[tc.serviceTag], value)
			}

			value, ok = env[tc.servicePort]
			if !ok {
				t.Fatalf("expected (%s) key, but was not found", tc.servicePort)
			}
			if ok && value != tc.keyValue[tc.servicePort] {
				t.Fatalf("values didn't match, expected (%s), got (%s)", tc.keyValue[tc.servicePort], value)
			}

		})
	}
}

func TestRunner_configEnv(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name      string
		env       map[string]string
		pristine  bool
		custom    []string
		whitelist []string
		blacklist []string
		output    map[string]string
	}{
		{
			name:     "pristine env with no custom vars yields empty env",
			env:      map[string]string{"PATH": "/bin"},
			pristine: true,
			output:   map[string]string{},
		},
		{
			name:     "pristine env with custom vars only keeps custom vars",
			env:      map[string]string{"PATH": "/bin"},
			pristine: true,
			custom:   []string{"GOPATH=/usr/go"},
			output:   map[string]string{"GOPATH": "/usr/go"},
		},
		{
			name:   "custom vars overwrite input vars",
			env:    map[string]string{"PATH": "/bin"},
			custom: []string{"PATH=/usr/bin"},
			output: map[string]string{"PATH": "/usr/bin"},
		},
		{
			name:      "whitelist filters input by key",
			env:       map[string]string{"GOPATH": "/usr/go", "GO111MODULES": "true", "PATH": "/bin"},
			whitelist: []string{"GO*"},
			output:    map[string]string{"GOPATH": "/usr/go", "GO111MODULES": "true"},
		},
		{
			name:      "blacklist takes precedence over whitelist",
			env:       map[string]string{"GOPATH": "/usr/go", "PATH": "/bin", "EDITOR": "vi"},
			whitelist: []string{"GO*", "EDITOR"},
			blacklist: []string{"GO*"},
			output:    map[string]string{"EDITOR": "vi"},
		},
		{
			name:      "custom takes precedence over blacklist",
			env:       map[string]string{"PATH": "/bin", "EDITOR": "vi"},
			blacklist: []string{"EDITOR*"},
			custom:    []string{"EDITOR=nvim"},
			output:    map[string]string{"EDITOR": "nvim", "PATH": "/bin"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{
				Exec: &config.ExecConfig{
					Env: &config.EnvConfig{
						Pristine:  &tc.pristine,
						Blacklist: tc.blacklist,
						Whitelist: tc.whitelist,
						Custom:    tc.custom,
					},
				},
			}
			c := DefaultConfig().Merge(&cfg)
			r, err := NewRunner(c, true)
			if err != nil {
				t.Fatal(err)
			}
			result := r.applyConfigEnv(tc.env)

			if !reflect.DeepEqual(result, tc.output) {
				t.Fatalf("expected: %v\n got: %v", tc.output, result)
			}
		})
	}
}
