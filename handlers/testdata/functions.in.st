function max2: int
max2:=a;
end_function
function_block motor
var_input
cmd: bool;
speed: real;
end_var
method Execute: bool
var
result: bool;
end_var
result:=cmd;
end_method
action do_it
speed:=10.0;
end_action
end_function_block
function combine: int
combine:=max2(1,2);
end_function