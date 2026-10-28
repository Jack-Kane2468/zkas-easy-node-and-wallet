package telegrambot

import (
	"encoding/json"
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"unsafe"
)

// DPAPI binds the token, viewing keys and allowlist to this Windows user.
func crypt(data []byte, protect bool) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("Empty bot settings")
	}
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	var err error
	if protect {
		err = windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		err = windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}
	if err != nil {
		return nil, errors.New("Windows could not protect or unlock bot settings for this user")
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, int(out.Size))...), nil
}
func Save(root string, c Config) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	defer func() { clear(b) }()
	encrypted, err := crypt(b, true)
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "telegram-bot")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "settings-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(encrypted); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(dir, "settings.bin"))
}
func Load(root string) (Config, error) {
	c := Config{OwnerOnly: true}
	b, err := os.ReadFile(filepath.Join(root, "telegram-bot", "settings.bin"))
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	plain, err := crypt(b, false)
	if err != nil {
		return c, err
	}
	defer clear(plain)
	err = json.Unmarshal(plain, &c)
	return c, err
}
