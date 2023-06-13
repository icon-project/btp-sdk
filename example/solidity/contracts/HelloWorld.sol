// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

contract HelloWorld {
    string private _value;

    constructor(string memory value){
        _value = value;
    }

    function name(
    ) external view returns (
        string memory
    ) {
        return _value;
    }

    function setName(
        string memory name
    ) external {
        _value = name;
        emit HelloEvent(name);
    }

    event HelloEvent(string name);

    function callInteger(
        int8 arg1,
        int16 arg2,
        int32 arg3,
        int64 arg4,
        int24 arg5,
        int40 arg6,
        int72 arg7,
        int arg8
    ) external view returns (
        Integers memory
    ){
        return Integers(arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8);
    }

    struct Integers {
        int8 arg1;
        int16 arg2;
        int32 arg3;
        int64 arg4;
        int24 arg5;
        int40 arg6;
        int72 arg7;
        int arg8;
    }

    function callUnsignedInteger(
        uint8 arg1,
        uint16 arg2,
        uint32 arg3,
        uint64 arg4,
        uint24 arg5,
        uint40 arg6,
        uint72 arg7,
        uint arg8
    ) external view returns (
        UnsignedIntegers memory
    ){
        return UnsignedIntegers(arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8);
    }

    struct UnsignedIntegers {
        uint8 arg1;
        uint16 arg2;
        uint32 arg3;
        uint64 arg4;
        uint24 arg5;
        uint40 arg6;
        uint72 arg7;
        uint arg8;
    }

    function callPrimitive(
        int arg1,
        bool arg2,
        string memory arg3,
        bytes memory arg4,
        address arg5
    ) external view returns (
        Primitives memory
    ){
        return Primitives(arg1, arg2, arg3, arg4, arg5);
    }

    struct Primitives {
        int arg1;
        bool arg2;
        string arg3;
        bytes arg4;
        address arg5;
    }

    function callStruct(
        InputStruct memory arg1
    ) external view returns (
        OutputStruct memory
    ) {
        return OutputStruct(arg1.booleanVal);
    }

    struct InputStruct {
        bool booleanVal;
    }

    struct OutputStruct {
        bool booleanVal;
    }

    function callArray(
        int[] memory arg1,
        bool[] memory arg2,
        string[] memory arg3,
        bytes[] memory arg4,
        address[] memory arg5,
        InputStruct[] memory arg6
    ) external view returns (
        Arrays memory
    ) {
        return Arrays(arg1, arg2, arg3, arg4, arg5, arg6);
    }

    struct Arrays {
        int[] arg1;
        bool[] arg2;
        string[] arg3;
        bytes[] arg4;
        address[] arg5;
        InputStruct[] arg6;
    }

    function callOptional(
    ) external view returns (
        string memory
    ) {
        return "callOptional()";
    }

    function callOptional(
        string memory arg1
    ) external view returns (
        string memory
    ) {
        return string.concat("callOptional(",arg1,")");
    }

    function invokeInteger(
        int8 arg1,
        int16 arg2,
        int32 arg3,
        int64 arg4,
        int24 arg5,
        int40 arg6,
        int72 arg7,
        int arg8
    ) external {
        emit IntegerEvent(arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8);
    }
    event IntegerEvent(
        int8 indexed arg1,
        int16 indexed arg2,
        int32 indexed arg3,
        int64 arg4,
        int24 arg5,
        int40 arg6,
        int72 arg7,
        int arg8
    );

    function invokePrimitive(
        int arg1,
        bool arg2,
        string memory arg3,
        bytes memory arg4,
        address arg5
    ) external {
        emit PrimitiveEvent(arg1, arg2, arg3, arg4, arg5);
    }
    event PrimitiveEvent(
        int indexed arg1,
        bool indexed arg2,
        string indexed arg3,
        bytes arg4,
        address arg5);

    function invokeStruct(
        InputStruct memory arg1
    ) external returns (
        OutputStruct memory
    ) {
        OutputStruct memory out = OutputStruct(arg1.booleanVal);
        emit StructEvent(out);
        return out;
    }
    event StructEvent(OutputStruct indexed arg1);

    function invokeArray(
        int[] memory arg1,
        bool[] memory arg2,
        string[] memory arg3,
        bytes[] memory arg4,
        address[] memory arg5,
        InputStruct[] memory arg6
    ) external {
        OutputStruct[] memory _arg6 = new OutputStruct[](arg6.length);
        for (uint256 i = 0; i < arg6.length; i++) {
            _arg6[i] = OutputStruct(arg6[i].booleanVal);
        }
        emit ArrayEvent(arg1, arg2, arg3, arg4, arg5, _arg6);
    }
    event ArrayEvent(
        int[] indexed arg1,
        bool[] indexed arg2,
        string[] indexed arg3,
        bytes[] arg4,
        address[] arg5,
        OutputStruct[] arg6);
}
