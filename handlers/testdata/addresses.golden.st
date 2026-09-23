PROGRAM p
VAR
	inBit    : BOOL AT %i*;
	outBit   : BOOL AT %q*;
	inByte   : BYTE AT %ib3;
	inWord   : WORD AT %IW50;
	outBit2  : BOOL AT %Q0.0;
	memWord  : WORD AT %MW100;
	memByte  : BYTE AT %MB128;
	memDword : DWORD AT %MD1000;
	memBit   : BOOL AT %M0.3;
END_VAR
END_PROGRAM
