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

package foundation.icon.gradle.plugins.btp.task;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import foundation.icon.btp.generator.ContractSpec;
import foundation.icon.btp.generator.ContractSpecGenerator;
import org.gradle.api.DefaultTask;
import org.gradle.api.file.RegularFileProperty;
import org.gradle.api.provider.Property;
import org.gradle.api.tasks.Input;
import org.gradle.api.tasks.InputFile;
import org.gradle.api.tasks.OutputFile;
import org.gradle.api.tasks.TaskAction;

import java.io.IOException;
import java.nio.file.Files;

abstract public class GenerateContractSpecTask extends DefaultTask {
    @InputFile
    abstract public RegularFileProperty getJarFile();

    @OutputFile
    abstract public RegularFileProperty getContractSpecFile();

    @Input
    abstract public Property<String> getSpecVersion();

    @TaskAction
    public void process() throws IOException {
        byte[] jarBytes = Files.readAllBytes(getJarFile().get().getAsFile().toPath());
        var out = getContractSpecFile().get().getAsFile().toPath();
        String specVersion = getSpecVersion().getOrNull();
        ContractSpec value = ContractSpecGenerator.generateFromJar(jarBytes);
        ObjectMapper mapper = new ObjectMapper().enable(SerializationFeature.INDENT_OUTPUT);
        mapper.writeValue(out.toFile(), value);
        System.out.println("generated "+out);
    }
}
