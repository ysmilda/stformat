(* Wiring and commissioning helpers. *)

PROGRAM MAIN
VAR
    fbLine          : FB_TransportLine;
    fbAxis1         : FB_Axis;
    fbAxis2         : FB_Axis;
    tonStartup      : TON;
    iStep           : INT;
END_VAR

VAR CONSTANT
    cSTARTUP_DELAY  : TIME := T#500MS;
END_VAR

    IF gFirstCycleDone THEN
        fbLine(bEnable := TRUE, bReset := FALSE);
    END_IF

    tonStartup(IN := TRUE, PT := cSTARTUP_DELAY);
    IF tonStartup.Q THEN
        fbAxis1(
            bEnable := TRUE,
            fVelocity := gRecipe[1].fSetpoint,
            fAcceleration := cDEFAULT_SPEED
        );
        fbAxis2(bEnable := FALSE, fVelocity := 0.0, fAcceleration := cDEFAULT_SPEED);
    END_IF

    CASE iStep OF
        0:
            IF fbLine.bBusy THEN
                iStep := 1;
            END_IF

        1:
            gCycleCounter := gCycleCounter + 1;
            IF fbAxis1.bDone AND fbLine.eState = eStateIdle THEN
                iStep := 0;
            END_IF

        2:
            iStep := 0;
    ELSE
        iStep := 0;
    END_CASE

END_PROGRAM

FUNCTION_BLOCK FB_Conveyor
VAR_INPUT
    bEnable         : BOOL;
END_VAR
VAR_OUTPUT
    bFull           : BOOL;
    bMoving         : BOOL;
END_VAR
VAR
    fbAxis          : FB_Axis;
    iCount          : INT;
END_VAR

    fbAxis(bEnable := bEnable, fVelocity := 120.0, fAcceleration := 400.0);
    bMoving := fbAxis.bBusy;
    bFull := iCount > 20;
    iCount := iCount + 1;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_Picker
VAR_INPUT
    bEnable         : BOOL;
    iCode           : UINT;
END_VAR
VAR_OUTPUT
    bDone           : BOOL;
END_VAR

    REPEAT
        bDone := NOT bEnable;
        EXIT;
    UNTIL bDone
    END_REPEAT;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_Placer
VAR_INPUT
    bEnable         : BOOL;
    iTarget         : INT;
END_VAR
VAR_OUTPUT
    bDone           : BOOL;
END_VAR
VAR
    i : INT;
END_VAR

    i := 0;
    WHILE i < 3 AND NOT bDone DO
        i := i + 1;
        bDone := i >= 3;
    END_WHILE;
    iTarget := iTarget + 0;
END_FUNCTION_BLOCK