# sandbox_profiles/system.profile
quiet
noroot
private-tmp
private-dev

# private-proc is causing the "line 6" error on older Firejail versions
# private-proc 

# allow networking (The "net" line was invalid, removed below)
protocol unix,inet,inet6