(* Sequence controller for the transport line. *)
(* The machine runs three stations in parallel and hands work between them. *)

FUNCTION_BLOCK FB_TransportLine
VAR_INPUT
    bEnable         : BOOL;
    bReset          : BOOL;
    tCycleTimeout   : TIME := T#10S;
    uiProductCode   : UINT;
END_VAR

VAR_OUTPUT
    eState          : E_LineState;
    bBusy           : BOOL;
    iUnitsProduced  : INT;
END_VAR

VAR
    tonCycle        : TON;
    tonStation1     : TON;
    tonStation2     : TON;
    tofJam          : TOF;
    fbPick          : FB_Picker;
    fbPlace         : FB_Placer;
    fbConveyor      : FB_Conveyor;
    arrStationState : ARRAY[1..3] OF BOOL;
    stStatus        : ST_LineStatus;
    iNextStation    : INT := 1;
    iRetryCount     : INT;
    sLastFault      : STRING(80);
END_VAR

VAR CONSTANT
    cMAX_RETRIES    : INT := 3;
    cSTATION_COUNT  : INT := 3;
END_VAR

METHOD PUBLIC Cycle : BOOL
VAR_INPUT
    bManual         : BOOL;
END_VAR
VAR_OUTPUT
    eResult         : E_Result;
END_VAR

    tonCycle(IN := bEnable, PT := tCycleTimeout);

    IF tonCycle.Q THEN
        eResult := eResultTimeout;
        Cycle := FALSE;
        RETURN;
    END_IF

    IF bReset THEN
        iRetryCount := 0;
        iUnitsProduced := 0;
        sLastFault := '';
        FOR iNextStation := 1 TO cSTATION_COUNT DO
            arrStationState[iNextStation] := FALSE;
        END_FOR
        tonStation1(IN := FALSE);
        tonStation2(IN := FALSE);
        eResult := eResultOk;
        Cycle := TRUE;
        RETURN;
    END_IF

    // Pick a product and put it on the belt.
    fbPick(bEnable := bEnable AND NOT fbConveyor.bFull, iCode := uiProductCode);

    IF fbPick.bDone THEN
        iRetryCount := 0;
        fbConveyor(bEnable := TRUE);
    END_IF

    IF fbConveyor.bFull THEN
        fbPlace(bEnable := TRUE, iTarget := iNextStation);
    END_IF

    IF fbPlace.bDone THEN
        iUnitsProduced := iUnitsProduced + 1;
        arrStationState[iNextStation] := TRUE;
        iNextStation := iNextStation + 1;
        IF iNextStation > cSTATION_COUNT THEN
            iNextStation := 1;
            eState := eStateIdle;
        END_IF
    END_IF

    tofJam(IN := NOT fbConveyor.bMoving, PT := T#2S);
    IF tofJam.Q THEN
        iRetryCount := iRetryCount + 1;
        sLastFault := CONCAT('Jammed at station ', INT_TO_STRING(iNextStation));
        IF iRetryCount >= cMAX_RETRIES THEN
            eResult := eResultFault;
            eState := eStateFault;
            Cycle := FALSE;
            RETURN;
        END_IF
        tofJam(IN := FALSE);
    END_IF

    stStatus.iProduced := iUnitsProduced;
    stStatus.sFault := sLastFault;
    bBusy := eState <> eStateIdle;
    eResult := eResultOk;
    Cycle := TRUE;
END_METHOD

METHOD PUBLIC Reset : BOOL
VAR
    i : INT;
END_VAR

    bReset := TRUE;
    fbPick.Reset();
    fbPlace.Reset();
    fbConveyor.Reset();
    FOR i := 1 TO cSTATION_COUNT DO
        arrStationState[i] := FALSE;
    END_FOR
    Reset := TRUE;
END_METHOD

METHOD PUBLIC GetStatus : ST_LineStatus
VAR_INPUT
    byRef : REFERENCE TO ST_LineStatus;
END_VAR

    byRef^ := stStatus;
    GetStatus := stStatus;
END_METHOD

PROPERTY PUBLIC UnitsProduced : INT
    Get
        UnitsProduced := iUnitsProduced;
    End_Get
END_PROPERTY

END_FUNCTION_BLOCK