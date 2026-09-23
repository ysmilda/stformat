CLASS my_class EXTENDS base
VAR
	x : INT;
END_VAR

METHOD public compute : INT
compute := x * 2;
END_METHOD
METHOD protected helper : INT
helper := THIS.x + SUPER.get();
END_METHOD
METHOD abstract enforce
END_METHOD
METHOD final lockit
register := 1;
END_METHOD
PROPERTY private value : INT
END_PROPERTY
END_CLASS

INTERFACE my_interface
METHOD public cycle : INT
END_METHOD
METHOD protected semi : INT
END_METHOD
END_INTERFACE

FUNCTION_BLOCK axis IMPLEMENTS i_motion
VAR
	pos : INT;
END_VAR

METHOD public move : INT
move := pos;
END_METHOD
END_FUNCTION_BLOCK
