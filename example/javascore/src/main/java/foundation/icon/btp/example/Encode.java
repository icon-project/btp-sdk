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

package foundation.icon.btp.example;

import score.Address;
import score.ByteArrayObjectWriter;
import score.Context;

import java.math.BigInteger;

public class Encode {
    public static byte[] encode(HelloWorld.OutputStruct obj) {
        ByteArrayObjectWriter w = Context.newByteArrayObjectWriter("RLPn");
        w.writeListOf(obj.getBooleanVal());
        return w.toByteArray();
    }

    public static byte[] encode(BigInteger[] arr) {
        ByteArrayObjectWriter w = Context.newByteArrayObjectWriter("RLPn");
        w.beginList(arr.length);
        for (BigInteger v : arr) {
            w.write(v);
        }
        w.end();
        return w.toByteArray();
    }

    public static byte[] encode(boolean[] arr) {
        ByteArrayObjectWriter w = Context.newByteArrayObjectWriter("RLPn");
        w.beginList(arr.length);
        for (boolean v : arr) {
            w.write(v);
        }
        w.end();
        return w.toByteArray();
    }

    public static byte[] encode(String[] arr) {
        ByteArrayObjectWriter w = Context.newByteArrayObjectWriter("RLPn");
        w.beginList(arr.length);
        for (String v : arr) {
            w.write(v);
        }
        w.end();
        return w.toByteArray();
    }

    public static byte[] encode(byte[][] arr) {
        ByteArrayObjectWriter w = Context.newByteArrayObjectWriter("RLPn");
        w.beginList(arr.length);
        for (byte[] v : arr) {
            w.write(v);
        }
        w.end();
        return w.toByteArray();
    }

    public static byte[] encode(Address[] arr) {
        ByteArrayObjectWriter w = Context.newByteArrayObjectWriter("RLPn");
        w.beginList(arr.length);
        for (Address v : arr) {
            w.write(v);
        }
        w.end();
        return w.toByteArray();
    }

    public static byte[] encode(HelloWorld.OutputStruct[] arr) {
        ByteArrayObjectWriter w = Context.newByteArrayObjectWriter("RLPn");
        w.beginList(arr.length);
        for (HelloWorld.OutputStruct v : arr) {
            w.writeListOf(v.getBooleanVal());
        }
        w.end();
        return w.toByteArray();
    }
}
