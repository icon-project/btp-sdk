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

package foundation.icon.gradle.plugins.btp;

import foundation.icon.gradle.plugins.btp.task.GenerateContractSpecTask;
import foundation.icon.gradle.plugins.javaee.JavaeePlugin;
import org.gradle.api.Project;
import org.gradle.jvm.tasks.Jar;

public class BTPPlugin extends JavaeePlugin {
    @Override
    public void apply(Project project) {
        super.apply(project);
        project.getTasks().register("generateContractSpec", GenerateContractSpecTask.class, (task) -> {
            Jar jarTask = (Jar)project.getTasks().getByName("jar");
            task.dependsOn(jarTask);
            task.getSpecVersion().value("V1");
            task.getJarFile().set(jarTask.getArchiveFile().get());
            task.getContractSpecFile().set(project.getBuildDir().toPath()
                    .resolve("generated").resolve("contractSpec.json").toFile());
        });
    }
}
