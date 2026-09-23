class my_class extends base
var
x: int;
end_var
method public compute: int
compute:=x*2;
end_method
method protected helper: int
helper:=this.x + super.get();
end_method
method abstract enforce
end_method
method final lockit
register:=1;
end_method
property private value: int
end_property
end_class
interface my_interface
method public cycle: int
end_method
method protected semi: int
end_method
end_interface
function_block axis implements i_motion
var
pos: int;
end_var
method public move: int
move:=pos;
end_method
end_function_block