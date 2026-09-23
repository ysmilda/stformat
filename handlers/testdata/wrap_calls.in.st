PROGRAM WrapCalls
VAR
    result : BOOL;

    // Array literal whose elements are parenthesized constructor calls.
    // The closing ']' must be preserved when the line overflows.
    slots : ARRAY[0..3] OF ST_SlotConfig[
        (key := 'aaa001', description := 'slot for the alpha lane'),
        (key := 'bbb002', description := 'slot for the beta side'),
        (key := 'ccc003', description := 'slot near the gamma unit'),
        (key := 'ddd004', description := 'slot below the delta frame')];
END_VAR

// A call that is not the end of the statement: the trailing
// `AND result` must survive long-line wrapping.
createStatusEventList := m_rejectList.addRejectReason(status := GVL_FlowStatus.INVALID, pEvent := ADR(m_statusUnknown), reasonCode := eReasonCode.Unreadable) AND createStatusEventList;
END_PROGRAM