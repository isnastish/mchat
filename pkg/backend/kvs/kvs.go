// Use my custom kvs package to test a key-value storage
// Settings is only the thing available in the package currently
package kvs

import (
	"github.com/isnastish/kvs/pkg/client"
)

type customBackend struct {
	settings *kvs.Settings
}
