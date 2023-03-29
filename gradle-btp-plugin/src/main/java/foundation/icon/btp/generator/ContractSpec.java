package foundation.icon.btp.generator;

import com.fasterxml.jackson.annotation.JsonIgnore;
import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import groovyjarjarantlr4.v4.runtime.misc.NotNull;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;
import lombok.RequiredArgsConstructor;

import java.util.List;

@Data
public class ContractSpec {
    private String name;
    private List<MethodSpec> methods;
    private List<EventSpec> events;
    private List<StructSpec> structs;

    @Data
    public static class MethodSpec {
        private String name;
        private List<NameAndTypeSpec> inputs;
        private TypeSpec output;
        @JsonInclude(JsonInclude.Include.NON_NULL)
        private Boolean readOnly;
        @JsonInclude(JsonInclude.Include.NON_NULL)
        private Boolean payable;
    }
    @Data
    public static class EventSpec {
        private String name;
        @JsonInclude(JsonInclude.Include.NON_NULL)
        private Integer indexed;
        private List<NameAndTypeSpec> inputs;
    }

    @Data
    public static class StructSpec {
        private String name;
        @JsonProperty(required = true)
        private List<NameAndTypeSpec> fields;
    }

    @Data
    public static class NameAndTypeSpec {
        private String name;
        private TypeSpec type;
    }

    @NoArgsConstructor
    @AllArgsConstructor
    @Data
    public static class TypeSpec {
        //FIXME Address?
        public enum TypeID {Unknown, Void, Integer, Boolean, String, Bytes, Struct, Address};
        @NotNull
        @JsonIgnore
        private TypeID typeID;
        @JsonInclude(JsonInclude.Include.NON_NULL)
        private Integer dimension;
        //[]TypeID
        @NotNull
        private String name;
    }
}
