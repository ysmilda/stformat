(* Numeric conversions, literals and time handling. *)

FUNCTION F_Scale : LREAL
VAR_INPUT
    rRaw            : INT;
    iRawMin         : INT := 0;
    iRawMax         : INT := 32767;
    fEngMin         : LREAL := 0.0;
    fEngMax         : LREAL := 100.0;
END_VAR
VAR
    fSpan           : LREAL;
END_VAR

    IF iRawMax = iRawMin THEN
        F_Scale := fEngMin;
        RETURN;
    END_IF

    fSpan := INT_TO_LREAL(iRawMax - iRawMin);
    F_Scale := fEngMin + ((INT_TO_LREAL(rRaw) - INT_TO_LREAL(iRawMin)) / fSpan) * (fEngMax - fEngMin);
END_FUNCTION

FUNCTION F_TimeToMs : DWORD
VAR_INPUT
    tValue          : TIME;
END_VAR

    F_TimeToMs := TIME_TO_DWORD(tValue) / DWORD#1000;
END_FUNCTION

FUNCTION F_BitPattern : DWORD
VAR_INPUT
    byRef           : REFERENCE TO BYTE;
    iCount          : INT := 4;
END_VAR
VAR
    dwResult        : DWORD;
    i               : INT;
END_VAR

    dwResult := 0;
    FOR i := 0 TO iCount - 1 DO
        dwResult := dwResult OR DWORD(SHL(UINT#16, INT_TO_UINT(i)));
    END_FOR
    F_BitPattern := dwResult;
END_FUNCTION

FUNCTION F_FormatMessage : STRING(80)
VAR_INPUT
    nCode           : INT;
    sDetail         : STRING(32);
END_VAR

    CASE nCode OF
        1:
            F_FormatMessage := CONCAT('Timeout: ', sDetail);
        2:
            F_FormatMessage := CONCAT('Soft limit: ', sDetail);
        3, 4:
            F_FormatMessage := CONCAT('Axis fault: ', sDetail);
        101:
            F_FormatMessage := CONCAT('Jammed: ', sDetail);
        102:
            F_FormatMessage := 'Not homed';
    ELSE
        F_FormatMessage := sDetail;
    END_CASE
END_FUNCTION

FUNCTION_BLOCK FB_TimerBank
VAR_INPUT
    tPeriod         : TIME := T#100MS;
END_VAR
VAR_OUTPUT
    arrElapsed      : ARRAY[0..7] OF TIME;
    iIndex          : INT;
END_VAR
VAR
    ton             : ARRAY[0..7] OF TON;
END_VAR

    iIndex := iIndex + 1;
    IF iIndex > 7 THEN
        iIndex := 0;
    END_IF

    ton[iIndex](IN := TRUE, PT := tPeriod);
    IF ton[iIndex].Q THEN
        arrElapsed[iIndex] := ton[iIndex].ET;
    END_IF
END_FUNCTION_BLOCK

FUNCTION F_AddressValid : BOOL
VAR_INPUT
    sAddress        : STRING(16);
END_VAR
VAR
    adr             : AT %IX0.0;
    p               : POINTER TO BYTE;
END_VAR

    F_AddressValid := sAddress <> '';
    adr.0 := FALSE;
    p := ADR(adr);
END_FUNCTION