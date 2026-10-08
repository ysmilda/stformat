(* Commissioning and diagnostics screen logic. Long argument lists and long
   guard conditions are normal in this code, but they are the exception rather
   than the rule. *)

FUNCTION_BLOCK FB_Diagnostics
VAR_INPUT
    iAxis           : INT;
    fPosition       : LREAL;
    fTarget         : LREAL;
    fVelocity       : LREAL;
    bHomed          : BOOL;
    bBusy           : BOOL;
    nError          : INT;
END_VAR
VAR_OUTPUT
    sMessage        : STRING(80);
    bAlarm          : BOOL;
END_VAR
VAR
    fbStop          : MC_Stop;
    fbMoveRelative  : MC_MoveRelative;
    tofOvertravel   : TOF;
END_VAR

    // One long guard: every condition has to hold before a restart is allowed.
    IF bHomed AND NOT bBusy AND nError = 0 AND ABS(fPosition - fTarget) < 0.05 AND fVelocity < 0.001 AND iAxis > 0 AND iAxis <= cAXIS_COUNT THEN
        fbStop(bExecute := TRUE, bDecel := TRUE);
        bAlarm := FALSE;
    END_IF

    tofOvertravel(IN := ABS(fPosition - fTarget) > 1.0, PT := T#2S);

    IF tofOvertravel.Q THEN
        fbMoveRelative(
            bExecute := TRUE,
            fDistance := (fTarget - fPosition) * 0.5,
            fVelocity := cDEFAULT_SPEED,
            fAcceleration := cDEFAULT_SPEED,
            fDecel := cDEFAULT_SPEED,
            bRelative := TRUE
        );
        sMessage := CONCAT('Axis ', INT_TO_STRING(iAxis), ' corrected by ', REAL_TO_STRING((fTarget - fPosition) * 0.5));
    END_IF

    // A full argument list is the other way these lines get long.
    fbStop(bExecute := FALSE, bDecel := FALSE, bImmediate := TRUE, fDecel := 0.0, tTimeout := T#0S, bForceAbort := FALSE);

    CASE nError OF
        101:
            sMessage := 'Jammed';
        102:
            sMessage := 'Not homed';
    ELSE
        IF bBusy THEN
            sMessage := 'Running';
        END_IF
    END_CASE
END_FUNCTION_BLOCK

FUNCTION F_AxisReport : STRING(160)
VAR_INPUT
    iAxis           : INT;
    fPosition       : LREAL;
    fTarget         : LREAL;
    bHomed          : BOOL;
    nError          : INT;
END_VAR

    IF nError <> 0 THEN
        F_AxisReport := CONCAT('Axis ', INT_TO_STRING(iAxis), ' error ', INT_TO_STRING(nError), ' at ', REAL_TO_STRING(fPosition));
        RETURN;
    END_IF

    F_AxisReport := CONCAT('Axis ', INT_TO_STRING(iAxis), ' pos ', REAL_TO_STRING(fPosition), ' target ', REAL_TO_STRING(fTarget), ' homed ', BOOL_TO_STRING(bHomed));
END_FUNCTION