program p
var_input
a: bool;
start_position: real := 1.0;
count: int;
end_var
var_output
result: string(32);
end_var
var_temp
tempbuf: array[1..10] of int;
end_var
var_global retain
max_speed: udint := 2000;
end_var
var
pin_ptr: pointer to int;
ref_var: reference to bool;
mapped: int at %mb30;
end_var
end_program