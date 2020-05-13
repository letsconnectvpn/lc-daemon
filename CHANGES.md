# ChangeLog

## 1.2.0 (...)
- to make it easier to package vpn-daemon use `const` for `tlsCaPath`, 
  `tlsCertPath` and `tlsKeyPath` and require patching the Go source if they 
  need to change from the defaults

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
