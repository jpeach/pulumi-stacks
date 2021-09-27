package conf

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// SSH helps generate a SSH client configuration file.
type SSH struct {
	configPath  string
	controlPath string
	lock        sync.Mutex
}

// NewSSH creates a new SSH client configuration file at path.
func NewSSH(path string) (*SSH, error) {
	configPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	controlPath := filepath.Join(filepath.Dir(configPath), ".control")
	if err := os.MkdirAll(controlPath, 0700); err != nil {
		return nil, err
	}

	fh, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0640)
	if err != nil {
		return nil, err
	}

	_ = fh.Close()

	s := SSH{
		configPath:  configPath,
		controlPath: controlPath,
	}

	s.append(func(fh *os.File) error {
		fh.WriteString("User fedora\n") // XXX(jpeach)
		fh.WriteString("StrictHostKeyChecking accept-new\n")
		fh.WriteString(fmt.Sprintf("UserKnownHostsFile %s\n",
			filepath.Join(filepath.Dir(configPath), "known_hosts")))

		return nil
	})

	return &s, nil
}

func (s *SSH) append(writer func(*os.File) error) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	fh, err := os.OpenFile(s.configPath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}

	defer fh.Close()

	return writer(fh)
}

// WriteBastionHost writes a host entry named "bastion". This is a SSH
// proxy host that workload host sessions will proxy through.
func (s *SSH) WriteBastionHost(address string, identity string) error {
	identity, err := filepath.Abs(identity)
	if err != nil {
		return err
	}

	return s.append(func(fh *os.File) error {
		_, err = fh.WriteString(
			fmt.Sprintf(`
Host bastion
  Hostname %s
  IdentityFile %s
  ControlMaster auto
  ControlPersist 5m
  ControlPath %s/%%r@%%h:%%p
`,
				address, identity, s.controlPath,
			))

		return err
	})
}

// WriteWorkloadHost writes a host entry that proxies through the bastion host.
func (s *SSH) WriteWorkloadHost(address string, identity string) error {
	identity, err := filepath.Abs(identity)
	if err != nil {
		return err
	}

	return s.append(func(fh *os.File) error {
		_, err = fh.WriteString(
			fmt.Sprintf(`
Host %s
  IdentityFile %s
  ProxyCommand ssh -F %s -W %%h:%%p bastion
`,
				address, identity, s.configPath,
			))

		return err
	})
}
