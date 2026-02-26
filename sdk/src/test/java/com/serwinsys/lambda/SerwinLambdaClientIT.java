package com.serwinsys.lambda;

import com.serwinsys.lambda.models.FunctionMetadata;
import com.serwinsys.lambda.models.InvokeResponse;
import com.serwinsys.lambda.models.LambdaArn;
import com.serwinsys.lambda.models.MetricsResponse;
import com.serwinsys.lambda.config.*;
import org.junit.jupiter.api.*;

import java.time.Duration;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * JUnit 5 Integration Tests for SerwinLambdaClient.
 * Tests run against a live server configured via environment variables.
 */
@TestInstance(TestInstance.Lifecycle.PER_CLASS)
public class SerwinLambdaClientIT {

    private SerwinLambdaClient client;
    private String functionName;
    private String functionArn;

    // "AKIAPQOQ22BFO8K2CT0Z",
    // "pjEspdB9IeF6o98fXrMaQcPeF5K/a/JTSop64vjF",
    //
    //

    @BeforeAll
    void setup() {
        String baseUrl = System.getenv().getOrDefault("LAMBDA_BASE_URL", "http://localhost:8053");
        functionName = System.getenv().getOrDefault("LAMBDA_TEST_FUNCTION", "hello-go");
        functionArn = System.getenv("LAMBDA_TEST_ARN");

        String accessKey = System.getenv().getOrDefault("LAMBDA_ACCESS_KEY", "AKIAPQOQ22BFO8K2CT0Z");
        String secretKey = System.getenv().getOrDefault("LAMBDA_SECRET_KEY",
                "pjEspdB9IeF6o98fXrMaQcPeF5K/a/JTSop64vjF");

        Assumptions.assumeTrue(baseUrl != null, "LAMBDA_BASE_URL must be set");

        SerwinCredentials credentials = new SerwinCredentials(accessKey, secretKey);
        client = new SerwinLambdaClient(baseUrl, credentials, Duration.ofSeconds(30));
    }

    @Test
    void testListFunctions() {
        List<FunctionMetadata> functions = client.listFunctions();
        assertNotNull(functions);
        System.out.println("Found " + functions.size() + " functions.");
    }

    @Test
    void testGetFunctionByName() {
        Assumptions.assumeTrue(functionName != null, "LAMBDA_TEST_FUNCTION must be set");
        FunctionMetadata metadata = client.getFunction(functionName);
        assertNotNull(metadata);
        assertEquals(functionName, metadata.getName());
    }

    @Test
    void testGetFunctionByArn() {
        Assumptions.assumeTrue(functionArn != null, "LAMBDA_TEST_ARN must be set");
        FunctionMetadata metadata = client.getFunctionByArn(functionArn);
        assertNotNull(metadata);
        assertEquals(functionArn, metadata.getArn());
    }

    @Test
    void testInvokeByName() {
        Assumptions.assumeTrue(functionName != null, "LAMBDA_TEST_FUNCTION must be set");
        InvokeResponse result = client.invoke(functionName, Map.of("key", "value"));
        assertNotNull(result);
        assertEquals("success", result.getStatus());
    }

    @Test
    void testInvokeByArn() {
        Assumptions.assumeTrue(functionArn != null, "LAMBDA_TEST_ARN must be set");
        InvokeResponse result = client.invokeByArn(functionArn, Map.of("key", "value"));
        assertNotNull(result);
        assertEquals("success", result.getStatus());
    }

    @Test
    void testGetMetricsByName() {
        Assumptions.assumeTrue(functionName != null, "LAMBDA_TEST_FUNCTION must be set");
        MetricsResponse metrics = client.getMetrics(functionName);
        assertNotNull(metrics);
    }

    @Test
    void testGetMetricsByArn() {
        Assumptions.assumeTrue(functionArn != null, "LAMBDA_TEST_ARN must be set");
        MetricsResponse metrics = client.getMetricsByArn(functionArn);
        assertNotNull(metrics);
    }

    @Test
    void testUpdateConfigByName() {
        Assumptions.assumeTrue(functionName != null, "LAMBDA_TEST_FUNCTION must be set");

        // Fetch current to verify update
        FunctionMetadata current = client.getFunction(functionName);
        assertNotNull(current);

        Map<String, Object> newConfig = Map.of(
                "memory", 256,
                "timeout", 30,
                "description", "Updated via JUnit IT");

        assertDoesNotThrow(() -> client.updateConfig(functionName, newConfig));

        FunctionMetadata updated = client.getFunction(functionName);
        assertEquals("Updated via JUnit IT", updated.getDescription());
    }

    @Test
    void testUpdateConfigByArn() {
        Assumptions.assumeTrue(functionArn != null, "LAMBDA_TEST_ARN must be set");

        Map<String, Object> newConfig = Map.of(
                "memory", 256,
                "timeout", 30,
                "description", "Updated via ARN JUnit IT");

        assertDoesNotThrow(() -> client.updateConfigByArn(functionArn, newConfig));

        FunctionMetadata updated = client.getFunctionByArn(functionArn);
        assertEquals("Updated via ARN JUnit IT", updated.getDescription());
    }

    @Test
    void testLambdaArnObjectOps() {
        Assumptions.assumeTrue(functionArn != null, "LAMBDA_TEST_ARN must be set");
        LambdaArn arnObj = LambdaArn.parse(functionArn);

        FunctionMetadata metadata = client.getFunctionByArn(arnObj);
        assertNotNull(metadata);
        assertEquals(functionArn, metadata.getArn());
    }
}
