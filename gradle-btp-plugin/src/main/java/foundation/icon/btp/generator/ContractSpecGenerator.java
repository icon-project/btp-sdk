/*
 * Copyright 2023 ICON Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package foundation.icon.btp.generator;

import foundation.icon.ee.struct.PropertyMember;
import foundation.icon.ee.struct.StructDB;
import foundation.icon.ee.tooling.deploy.OptimizedJarBuilder;
import foundation.icon.ee.types.Method;
import foundation.icon.ee.util.MethodUnpacker;
import org.aion.avm.utilities.JarBuilder;
import org.aion.avm.utilities.Utilities;
import org.objectweb.asm.ClassReader;
import org.objectweb.asm.ClassVisitor;
import org.objectweb.asm.MethodVisitor;
import org.objectweb.asm.Opcodes;
import org.objectweb.asm.Type;
import score.Address;

import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.math.BigInteger;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.jar.JarInputStream;
import java.util.stream.Collectors;

public class ContractSpecGenerator {
    public static ContractSpec generateFromJar(byte[] jarBytes) throws IOException {
        JarInputStream jarReader = new JarInputStream(new ByteArrayInputStream(jarBytes), true);
        String mainClassName = Utilities.extractMainClassName(jarReader, Utilities.NameStyle.DOT_NAME);
        Map<String, byte[]> classMap = Utilities.extractClasses(jarReader, Utilities.NameStyle.DOT_NAME);
        StructDB structDB = new StructDB(classMap);
        List<Method> methods = optimizedJarMethods(jarBytes);
        Map<String, Method> methodMap = methods.stream().collect(Collectors.toMap(Method::getName, v -> v));
        List<ContractSpec.MethodSpec> methodSpecs = new ArrayList<>();
        List<ContractSpec.EventSpec> eventSpecs = new ArrayList<>();
        Map<String, ContractSpec.StructSpec> structSpecMap = new HashMap<>();

        byte[] mainClassBytes = classMap.get(mainClassName);
        ClassReader cr = new ClassReader(mainClassBytes);
        cr.accept(new ClassVisitor(Opcodes.ASM7) {
            @Override
            public MethodVisitor visitMethod(int access, String name, String descriptor, String signature, String[] exceptions) {
                Method method = methodMap.get(name);
                if (method != null) {
                    System.out.printf("%s(", method.getName());
                    Method.Parameter[] inputs = method.getInputs();
                    Type[] argTypes = Type.getArgumentTypes(descriptor);
                    List<ContractSpec.NameAndTypeSpec> inputSpecs = new ArrayList<>();
                    for (int i = 0; i < argTypes.length; i++) {
                        Method.Parameter input = inputs[i];
                        Type argType = argTypes[i];
                        ContractSpec.NameAndTypeSpec fieldSpec = toNameAndTypeSpec(
                                structDB, structSpecMap, input.getName(), typeIDMap, argType, true);
                        if (input.isOptional()) {
                            fieldSpec.setOptional(true);
                        }
                        inputSpecs.add(fieldSpec);
                        System.out.printf("%s%s%s",
                                i > 0 ? "," : "",
                                input.isOptional() ? "@" : "",
                                fieldType[input.getType() & 0xF]);
                    }
                    Type returnType = Type.getReturnType(descriptor);
                    System.out.printf(")%s\n", fieldType[method.getOutput() & 0xF]);
                    switch (method.getType()) {
                        case Method.MethodType.FUNCTION:
                            ContractSpec.MethodSpec methodSpec = new ContractSpec.MethodSpec();
                            methodSpec.setName(method.getName());
                            methodSpec.setInputs(inputSpecs);
                            ContractSpec.TypeSpec outputSpec = returnType.getSort() == Type.VOID ?
                                    new ContractSpec.TypeSpec(ContractSpec.TypeSpec.TypeID.Void, null,
                                            ContractSpec.TypeSpec.TypeID.Void.name()) :
                                    toTypeSpec(structDB, structSpecMap, outputTypeIDMap, returnType, false);
                            methodSpec.setOutput(outputSpec);
                            if ((method.getFlags() & Method.Flags.READONLY) > 0) {
                                methodSpec.setReadOnly(true);
                            }
                            if ((method.getFlags() & Method.Flags.PAYABLE) > 0) {
                                methodSpec.setPayable(true);
                            }
                            methodSpecs.add(methodSpec);
                            break;
                        case Method.MethodType.EVENT:
                            ContractSpec.EventSpec eventSpec = new ContractSpec.EventSpec();
                            eventSpec.setName(method.getName());
                            eventSpec.setInputs(inputSpecs);
                            eventSpec.setIndexed(method.getIndexed());
                            eventSpecs.add(eventSpec);
                            break;
                    }
                }
                return super.visitMethod(access, name, descriptor, signature, exceptions);
            }
        }, 0);

        ContractSpec contractSpec = new ContractSpec();
        contractSpec.setName(mainClassName);
        contractSpec.setMethods(methodSpecs);
        contractSpec.setEvents(eventSpecs);
        contractSpec.setStructs(new ArrayList<>(structSpecMap.values()));
        return contractSpec;
    }

    static Map<String, ContractSpec.TypeSpec.TypeID> typeIDMap;
    static Map<String, ContractSpec.TypeSpec.TypeID> outputTypeIDMap;
    static {
        typeIDMap = new HashMap<>();
        typeIDMap.put(char.class.getTypeName(), ContractSpec.TypeSpec.TypeID.Integer);
        typeIDMap.put(byte.class.getTypeName(), ContractSpec.TypeSpec.TypeID.Integer);
        typeIDMap.put(short.class.getTypeName(), ContractSpec.TypeSpec.TypeID.Integer);
        typeIDMap.put(int.class.getTypeName(), ContractSpec.TypeSpec.TypeID.Integer);
        typeIDMap.put(long.class.getTypeName(), ContractSpec.TypeSpec.TypeID.Integer);
        typeIDMap.put(Character.class.getTypeName(), ContractSpec.TypeSpec.TypeID.Integer);
        typeIDMap.put(Byte.class.getTypeName(), ContractSpec.TypeSpec.TypeID.Integer);
        typeIDMap.put(Short.class.getTypeName(), ContractSpec.TypeSpec.TypeID.Integer);
        typeIDMap.put(Integer.class.getTypeName(), ContractSpec.TypeSpec.TypeID.Integer);
        typeIDMap.put(Long.class.getTypeName(), ContractSpec.TypeSpec.TypeID.Integer);
        typeIDMap.put(BigInteger.class.getTypeName(), ContractSpec.TypeSpec.TypeID.Integer);
        typeIDMap.put(boolean.class.getTypeName(), ContractSpec.TypeSpec.TypeID.Boolean);
        typeIDMap.put(Boolean.class.getTypeName(), ContractSpec.TypeSpec.TypeID.Boolean);
        typeIDMap.put(String.class.getTypeName(), ContractSpec.TypeSpec.TypeID.String);
        typeIDMap.put(byte[].class.getTypeName(), ContractSpec.TypeSpec.TypeID.Bytes);
        typeIDMap.put(Address.class.getTypeName(), ContractSpec.TypeSpec.TypeID.Address);

        outputTypeIDMap = new HashMap<>(typeIDMap);
        outputTypeIDMap.put(Map.class.getTypeName(), ContractSpec.TypeSpec.TypeID.Unknown);
        outputTypeIDMap.put(List.class.getTypeName(), ContractSpec.TypeSpec.TypeID.Unknown);
    }

    static ContractSpec.NameAndTypeSpec toNameAndTypeSpec(
            StructDB structDB, Map<String, ContractSpec.StructSpec> map,
            String name,
            Map<String, ContractSpec.TypeSpec.TypeID> typeIDMap, Type type, boolean in) {
        ContractSpec.NameAndTypeSpec nameAndTypeSpec = new ContractSpec.NameAndTypeSpec();
        nameAndTypeSpec.setName(name);
        nameAndTypeSpec.setType(toTypeSpec(structDB, map, typeIDMap, type, in));
        return nameAndTypeSpec;
    }

    static ContractSpec.TypeSpec toTypeSpec(
            StructDB structDB, Map<String, ContractSpec.StructSpec> map,
            Map<String, ContractSpec.TypeSpec.TypeID> typeIDMap, Type type, boolean in) {
        ContractSpec.TypeSpec typeSpec = new ContractSpec.TypeSpec();
        ContractSpec.TypeSpec.TypeID typeID;
        String typeName;
        if (type.getSort() == Type.VOID) {
            typeID = ContractSpec.TypeSpec.TypeID.Void;
            typeName = typeID.name();
        } else {
            if (type.getSort() == Type.ARRAY) {
                int dimensions = type.getDimensions();
                type = type.getElementType();
                if (type.getClassName().equals(byte.class.getTypeName())) {
                    type = Type.getType(byte[].class);
                    dimensions--;
                }
                if (dimensions > 0) {
                    typeSpec.setDimension(dimensions);
                }
            }
            typeID = typeIDMap.get(type.getClassName());
            if (typeID != null) {
                typeName = typeID.equals(ContractSpec.TypeSpec.TypeID.Unknown) ?
                        type.getClassName() : typeID.name();
            } else {
                typeID = ContractSpec.TypeSpec.TypeID.Struct;
                typeName = type.getClassName();
            }
        }
        typeSpec.setTypeID(typeID);
        typeSpec.setName(typeName);
        if (typeID.equals(ContractSpec.TypeSpec.TypeID.Struct)) {
            collectStructSpec(structDB, map, typeIDMap, type, in);
        }
        return typeSpec;
    }
    static void collectStructSpec(
            StructDB structDB, Map<String, ContractSpec.StructSpec> map,
            Map<String, ContractSpec.TypeSpec.TypeID> typeIDMap, Type type, boolean in) {
        String typeName = type.getClassName();
        if (!map.containsKey(typeName)) {
            ContractSpec.StructSpec structSpec = new ContractSpec.StructSpec();
            structSpec.setName(typeName);
            List<PropertyMember> props = in ? structDB.getWritableProperties(type) :
                    structDB.getReadableProperties(type);
            Objects.requireNonNull(props, "not exists properties");
            structSpec.setFields(props.stream()
                    .map(v -> toNameAndTypeSpec(structDB, map, v.getName(), typeIDMap, v.getType(), in))
                    .collect(Collectors.toList()));
            map.put(type.getClassName(), structSpec);
        }
    }

    static List<Method> optimizedJarMethods(byte[] jarBytes) throws IOException {
        byte[] optimizedJarBytes = new OptimizedJarBuilder(true, jarBytes)
                .withUnreachableMethodRemover()
                .withRenamer().getOptimizedBytes();
        byte[] apisBytes = JarBuilder.getAPIsBytesFromJAR(optimizedJarBytes);
        return Arrays.asList(MethodUnpacker.readFrom(apisBytes));
    }


    public static String[] methodType = new String[]{"function", "fallback", "eventlog"};
    public static String[] fieldType = new String[]{"void", "int", "str", "bytes", "bool", "Address", "list", "dict", "struct"};

}
