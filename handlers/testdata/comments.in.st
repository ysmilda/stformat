program p
VAR
x:int; //the counter
END_VAR
x:=1; //set x
(* block
comment *)
z:=2;
if x then
    y:=x; //branch
end_if //done
(* note *) z := 3;
end_program

// A comment standing on its own line inside a condition.
// The tokens around it ask for no line break, so a second pass used to glue it to the line above.
program p
IF a
// both must be set
AND b THEN c := 1; END_IF
end_program

// A trailing comment on a condition that has to wrap moves to the THEN line.
program p
IF flagOperationalReady AND flagMaintenanceOverride AND flagEmergencyStopActive AND flagPowerAvailable AND flagGateClosed AND flagInterlockEngaged AND flagSequenceRunning AND flagAbortRequested AND flagResetPermitted AND flagCycleComplete AND flagAutoModeSelected THEN // all flags
    x := TRUE;
END_IF
end_program

function_block fb extends base
// built by the comment test
var
    y:int;
end_var
y:=1;
end_function_block

// a type with the comment on the line that opens its body
type states :
(
// the state names
a:=0,
b:=1
);
end_type

function_block child public extends base
// inherits from base
var
    w:int;
end_var
end_function_block
(*****************)
//////////
///doc comment
//x := 1; commented-out code keeps its case
//   indented text keeps its indentation
