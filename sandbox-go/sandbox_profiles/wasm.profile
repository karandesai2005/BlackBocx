# Base restrictions
private
private-dev
private-tmp
nosound
no3d
nodbus
nogroups
noroot
seccomp

# Allow minimal binaries
whitelist /usr/bin
whitelist /bin
whitelist /usr/lib

# Network allowed (for nmap, gobuster)
netfilter
