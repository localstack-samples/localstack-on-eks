package dns

import (
	"reflect"
	"testing"
)

func TestUnmarshalAndMarshal(t *testing.T) {
	parser := &DefaultCorefileParser{}

	testCases := []struct {
		name string
		data string
	}{
		{
			name: "Test Case 1",
			data: `
.:53 {
    errors
    health {
        lameduck 5s
    }
    ready
    kubernetes cluster.local in-addr.arpa ip6.arpa {
        pods insecure
        fallthrough in-addr.arpa ip6.arpa
        ttl 30
    }
    prometheus :9153
    forward . /etc/resolv.conf {
        max_concurrent 1000
    }
    cache 30
    loop
    reload
    loadbalance
}

localstack0:53 {
    errors
    cache
    forward . 10.100.2.53 {}
}
`,
		},
		{
			name: "Test Case 2",
			data: `
.:53 {
    errors
    health {
        lameduck 5s
    }
    ready
    kubernetes cluster.local in-addr.arpa ip6.arpa {
        pods insecure
        fallthrough in-addr.arpa ip6.arpa
        ttl 30
    }
    prometheus :9153
    forward . /etc/resolv.conf {
        max_concurrent 1000
    }
    cache 30
    loop
    reload
    loadbalance
}
`,
		},
		{
			name: "Test Case 3",
			data: `
. {
	reload
	erratic
}
`,
		},
		{
			name: "Test Case 4",
			data: `
localstack0:53 {
	errors
	cache
	forward . 10.100.2.53 {}
}

localstack0:53 {
    errors
    cache
    forward . 10.100.2.53 {}
}
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config, err := parser.Unmarshal(tc.data)
			if err != nil {
				t.Fatalf("Error parsing Corefile: %v", err)
			}

			_, err = parser.Marshal(config)
			if err != nil {
				t.Fatalf("Error marshaling Corefile: %v", err)
			}
		})
	}
}

// Add test that tests all directive ops functions
func TestDirectiveOps(t *testing.T) {
	parser := &DefaultCorefileParser{}

	testCase := `
	.:53 {
		errors
		health {
			lameduck 5s
		}
		ready
		kubernetes cluster.local in-addr.arpa ip6.arpa {
			pods insecure
			fallthrough in-addr.arpa ip6.arpa
			ttl 30
		}
		prometheus :9153
		forward . /etc/resolv.conf {
			max_concurrent 1000
		}
		cache 30
		loop
		reload
		loadbalance
	}
	`

	config, err := parser.Unmarshal(testCase)
	if err != nil {
		t.Fatalf("Error parsing Corefile: %v", err)
	}

	// Add new directive
	directive := Directive{
		Name: "localstack0:53",
		Entries: []DirectiveEntry{
			{
				StrValue: "errors",
			},
			{
				StrValue: "cache",
			},
			{
				StrValue: "forward . 10.100.2.53 {}",
			},
		},
	}
	config.AddDirective(directive)

	// Marshal Corefile to test the new directive
	marshaled, err := parser.Marshal(config)
	if err != nil {
		t.Fatalf("Error marshaling Corefile: %v", err)
	}

	newConfig, err := parser.Unmarshal(marshaled)
	if err != nil {
		t.Fatalf("Error parsing Corefile: %v", err)
	}

	// Check that the new directive was added
	if len(newConfig.Directives) != 2 {
		t.Fatalf("Expected 2 directives, got %d", len(newConfig.Directives))
	}

	// Check that the new directive was added correctly
	if newConfig.Directives[1].Name != "localstack0:53" {
		t.Fatalf("Expected directive name to be localstack0:53, got %s", newConfig.Directives[1].Name)
	}

	// Check the names of the directives
	if !reflect.DeepEqual(newConfig.GetDirectiveNames(), []string{".:53", "localstack0:53"}) {
		t.Fatalf("Expected directive names to be [.53, localstack0:53], got %v", newConfig.GetDirectiveNames())
	}

	// Remove a non-existing directive
	if newConfig.RemoveDirective("non-existing-directive") != 0 {
		t.Fatalf("No directive is expected here")
	}

	// Remove the newly added directive
	if newConfig.RemoveDirective("localstack0:53") != 1 {
		t.Fatalf("Expected 1 directive to be removed")
	}

	// Test the KeepUniqueDirectives function
	newConfig.AddDirective(directive)
	newConfig.AddDirective(directive)
	if newConfig.KeepUniqueDirectives() != 1 {
		t.Fatalf("Expected 1 directive to be removed")
	}
	if newConfig.KeepUniqueDirectives() != 0 {
		t.Fatalf("No directive is expected here")
	}
}
