program p
var_inst
fbInner: fb_motor;
end_var
var_stat
callCount: int;
end_var
var_external
shared: int;
end_var
var_config
inst.address: bool at %i*;
end_var
var_access
nCount: gvl.nCount: dint read_write;
sSecret: gvl.sSecret: string read_only;
end_var
var_global non_retain
cfg: int;
end_var
var constant
kPi: real := 3.14159;
end_var
var persistent
bootCount: dint;
end_var
end_program