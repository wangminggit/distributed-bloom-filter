# Java 示例

本目录包含 Distributed Bloom Filter (DBF) 的 Java 客户端示例。

## 项目结构

```
java/
├── README.md           # 本文档
├── pom.xml             # Maven 配置
└── src/
    └── main/
        └── java/
            └── com/
                └── dbf/
                    ├── DBFClient.java      # DBF 客户端封装
                    └── Main.java           # 示例代码
```

## 快速开始

### 1. 克隆项目

```bash
git clone https://github.com/wangminggit/distributed-bloom-filter.git
cd distributed-bloom-filter/examples/java
```

### 2. 安装依赖

```bash
mvn install
```

### 3. 运行示例

```bash
mvn exec:java -Dexec.mainClass="com.dbf.Main"
```

## 完整示例代码

### DBFClient.java

```java
package com.dbf;

import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import com.dbf.proto.BloomFilterGrpc;
import com.dbf.proto.BloomFilterOuterClass;

public class DBFClient implements AutoCloseable {
    private final ManagedChannel channel;
    private final BloomFilterGrpc.BloomFilterBlockingStub stub;

    public DBFClient(String host, int port) {
        this.channel = ManagedChannelBuilder.forAddress(host, port)
            .usePlaintext()
            .build();
        this.stub = BloomFilterGrpc.newBlockingStub(channel);
    }

    public void add(String element) {
        BloomFilterOuterClass.AddRequest request = BloomFilterOuterClass.AddRequest.newBuilder()
            .addElement(element)
            .build();
        stub.add(request);
    }

    public boolean contains(String element) {
        BloomFilterOuterClass.ContainsRequest request = BloomFilterOuterClass.ContainsRequest.newBuilder()
            .addElement(element)
            .build();
        BloomFilterOuterClass.ContainsResponse response = stub.contains(request);
        return response.getExists();
    }

    public void delete(String element) {
        BloomFilterOuterClass.DeleteRequest request = BloomFilterOuterClass.DeleteRequest.newBuilder()
            .addElement(element)
            .build();
        stub.delete(request);
    }

    public void batchAdd(java.util.List<String> elements) {
        BloomFilterOuterClass.BatchAddRequest request = BloomFilterOuterClass.BatchAddRequest.newBuilder()
            .addAllElements(elements)
            .build();
        stub.batchAdd(request);
    }

    public java.util.List<Boolean> batchContains(java.util.List<String> elements) {
        BloomFilterOuterClass.BatchContainsRequest request = BloomFilterOuterClass.BatchContainsRequest.newBuilder()
            .addAllElements(elements)
            .build();
        BloomFilterOuterClass.BatchContainsResponse response = stub.batchContains(request);
        return response.getExistsList();
    }

    @Override
    public void close() {
        channel.shutdown();
    }
}
```

### Main.java

```java
package com.dbf;

import java.util.Arrays;
import java.util.List;

public class Main {
    public static void main(String[] args) {
        // 连接 DBF 集群
        try (DBFClient client = new DBFClient("localhost", 50051)) {
            
            // 添加单个元素
            client.add("user:12345");
            System.out.println("Added user:12345");
            
            // 查询元素
            boolean exists = client.contains("user:12345");
            System.out.println("Contains user:12345: " + exists);
            
            // 删除元素
            client.delete("user:12345");
            System.out.println("Deleted user:12345");
            
            // 再次查询
            exists = client.contains("user:12345");
            System.out.println("Contains user:12345 after delete: " + exists);
            
            // 批量操作
            List<String> users = Arrays.asList("user:1", "user:2", "user:3");
            client.batchAdd(users);
            System.out.println("Batch added: " + users);
            
            List<Boolean> results = client.batchContains(users);
            System.out.println("Batch contains results: " + results);
            
        } catch (Exception e) {
            System.err.println("Error: " + e.getMessage());
            e.printStackTrace();
        }
    }
}
```

## Maven 依赖 (pom.xml)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 
         http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>

    <groupId>com.dbf</groupId>
    <artifactId>dbf-java-example</artifactId>
    <version>1.0.0</version>
    <packaging>jar</packaging>

    <properties>
        <maven.compiler.source>11</maven.compiler.source>
        <maven.compiler.target>11</maven.compiler.target>
        <grpc.version>1.60.0</grpc.version>
        <protobuf.version>3.25.1</protobuf.version>
    </properties>

    <dependencies>
        <!-- gRPC -->
        <dependency>
            <groupId>io.grpc</groupId>
            <artifactId>grpc-netty-shaded</artifactId>
            <version>${grpc.version}</version>
        </dependency>
        <dependency>
            <groupId>io.grpc</groupId>
            <artifactId>grpc-protobuf</artifactId>
            <version>${grpc.version}</version>
        </dependency>
        <dependency>
            <groupId>io.grpc</groupId>
            <artifactId>grpc-stub</artifactId>
            <version>${grpc.version}</version>
        </dependency>

        <!-- Protobuf -->
        <dependency>
            <groupId>com.google.protobuf</groupId>
            <artifactId>protobuf-java</artifactId>
            <version>${protobuf.version}</version>
        </dependency>

        <!-- 注解 -->
        <dependency>
            <groupId>javax.annotation</groupId>
            <artifactId>javax.annotation-api</artifactId>
            <version>1.3.2</version>
        </dependency>
    </dependencies>

    <build>
        <extensions>
            <extension>
                <groupId>kr.motd.maven</groupId>
                <artifactId>os-maven-plugin</artifactId>
                <version>1.7.1</version>
            </extension>
        </extensions>
        <plugins>
            <plugin>
                <groupId>org.xolstice.maven.plugins</groupId>
                <artifactId>protobuf-maven-plugin</artifactId>
                <version>0.6.1</version>
                <configuration>
                    <protocArtifact>com.google.protobuf:protoc:${protobuf.version}:exe:${os.detected.classifier}</protocArtifact>
                    <pluginId>grpc-java</pluginId>
                    <pluginArtifact>io.grpc:protoc-gen-grpc-java:${grpc.version}:exe:${os.detected.classifier}</pluginArtifact>
                </configuration>
                <executions>
                    <execution>
                        <goals>
                            <goal>compile</goal>
                            <goal>compile-custom</goal>
                        </goals>
                    </execution>
                </executions>
            </plugin>
            <plugin>
                <groupId>org.codehaus.mojo</groupId>
                <artifactId>exec-maven-plugin</artifactId>
                <version>3.1.0</version>
            </plugin>
        </plugins>
    </build>
</project>
```

## 生成 Protobuf 代码

在运行 `mvn install` 时，Maven 会自动从 DBF 服务的 `.proto` 文件生成 Java 代码。

如果需要手动生成：

```bash
# 下载 proto 文件
curl -O https://raw.githubusercontent.com/wangminggit/distributed-bloom-filter/main/proto/bloomfilter.proto

# 使用 protoc 生成
protoc --java_out=src/main/java --grpc-java_out=src/main/java \
  --plugin=protoc-gen-grpc-java=$(which protoc-gen-grpc-java) \
  bloomfilter.proto
```

## 注意事项

1. **连接地址**: 示例中使用 `localhost:50051`，生产环境请替换为实际的 DBF 网关地址
2. **错误处理**: 生产环境应添加完善的错误处理和重试机制
3. **连接池**: 高并发场景建议使用连接池管理 DBFClient 实例
4. **超时配置**: 建议为 gRPC 调用配置合理的超时时间

## 更多信息

- 主项目 README: [../../README.md](../../README.md)
- API 文档: [../../docs/api.md](../../docs/api.md)
- 部署指南: [../../deploy/README.md](../../deploy/README.md)
