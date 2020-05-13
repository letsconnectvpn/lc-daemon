# ChangeLog

## 1.2.0 (...)
- allow specifying full path to `tlsCaPath`, `tlsCertPath` and `tlsKeyPath` 
  isntead of folders allowing for more flexibility and easier packaging

## 1.1.1 (2020-05-01)
- update `Makefile` to support `install`

## 1.1.0 (2020-04-16)
- also return management port when using `LIST` command to be able to link 
  client connections to a particular OpenVPN process

## 1.0.1 (2020-04-08)
- fix parsing of OpenVPN status commando when no IP addresses are set 
  (issue #53)

## 1.0.0 (2019-11-18)
- initial release
