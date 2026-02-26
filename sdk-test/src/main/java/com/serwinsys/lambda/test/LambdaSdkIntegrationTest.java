package com.serwinsys.lambda.test;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.serwinsys.lambda.SerwinLambdaClient;
import com.serwinsys.lambda.exceptions.LambdaException;
import com.serwinsys.lambda.models.FunctionMetadata;
import com.serwinsys.lambda.models.LambdaArn;
import com.serwinsys.lambda.models.MetricsResponse;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.List;
import java.util.Map;

/**
 * Comprehensive integration test suite for the Serwin Lambda SDK.
 *
 * <p>
 * <b>Required environment variables:</b>
 * 
 * <pre>
 *   LAMBDA_BASE_URL        – e.g. http://localhost:8080
 *   LAMBDA_API_KEY         – bearer token / API key
 *   LAMBDA_TEST_FUNCTION   – name of an existing function (e.g. "hello-world")
 *   LAMBDA_TEST_ARN        – ARN of that same function (e.g. arn:serwin:lambda:us-east-1:123:function:hello-world)
 * </pre>
 *
 * <p>
 * <b>Run:</b>
 * 
 * <pre>
 *   # First install the SDK locally
 *   cd ../sdk && mvn install -q
 *   # Then run the tests
 *   cd ../sdk-test && mvn compile exec:java
 * </pre>
 */
public class LambdaSdkIntegrationTest {

    // ── ANSI colours ─────────────────────────────────────────────────────────
    private static final String RESET = "\u001B[0m";
    private static final String GREEN = "\u001B[32m";
    private static final String RED = "\u001B[31m";
    private static final String CYAN = "\u001B[36m";
    private static final String YELLOW = "\u001B[33m";
    private static final String BOLD = "\u001B[1m";

    // ── Test state ────────────────────────────────────────────────────────────
    private int passed = 0;
    private int failed = 0;
    private int skipped = 0;

    private final SerwinLambdaClient client;
    private final String baseUrl;
    private final String apiKey;
    private final String functionName; // may be null → tests using it are skipped
    private final String functionArnRaw; // may be null → ARN tests are skipped

    public LambdaSdkIntegrationTest() {
        baseUrl = requireEnv("LAMBDA_BASE_URL");
        apiKey = requireEnv("LAMBDA_API_KEY");
        functionName = System.getenv("LAMBDA_TEST_FUNCTION");
        functionArnRaw = System.getenv("LAMBDA_TEST_ARN");

        client = new SerwinLambdaClient(baseUrl, apiKey, Duration.ofSeconds(60));
    }

    // ─────────────────────────────────────────────────────────────────────────
    // Entry point
    // ─────────────────────────────────────────────────────────────────────────

    public static void main(String[] args) {
        new LambdaSdkIntegrationTest().run();
    }

    public void run() {
        printBanner();

        // ── 1. Health check (raw HTTP, no auth required) ──────────────────────
        section("1. Health Check");
        testHealthCheck();

        // ── 2. List functions ─────────────────────────────────────────────────
        section("2. List Functions");
        testListFunctions();

        // ── 3. Get function by name ───────────────────────────────────────────
        section("3. Get Function by Name");
        if (functionName == null) {
            skipAll("LAMBDA_TEST_FUNCTION not set");
        } else {
            testGetFunctionByName();
        }

        // ── 4. Get function by ARN ────────────────────────────────────────────
        section("4. Get Function by ARN");
        if (functionArnRaw == null) {
            skipAll("LAMBDA_TEST_ARN not set");
        } else {
            testGetFunctionByArn();
        }

        // ── 5. Invoke by name ─────────────────────────────────────────────────
        section("5. Invoke by Name");
        if (functionName == null) {
            skipAll("LAMBDA_TEST_FUNCTION not set");
        } else {
            testInvokeByName();
        }

        // ── 6. Invoke by ARN ──────────────────────────────────────────────────
        section("6. Invoke by ARN");
        if (functionArnRaw == null) {
            skipAll("LAMBDA_TEST_ARN not set");
        } else {
            testInvokeByArn();
        }

        // ── 7. Get metrics by name ────────────────────────────────────────────
        section("7. Get Metrics by Name");
        if (functionName == null) {
            skipAll("LAMBDA_TEST_FUNCTION not set");
        } else {
            testGetMetricsByName();
        }

        // ── 8. Get metrics by ARN ─────────────────────────────────────────────
        section("8. Get Metrics by ARN");
        if (functionArnRaw == null) {
            skipAll("LAMBDA_TEST_ARN not set");
        } else {
            testGetMetricsByArn();
        }

        // ── 9. Update config by name ──────────────────────────────────────────
        section("9. Update Config by Name");
        if (functionName == null) {
            skipAll("LAMBDA_TEST_FUNCTION not set");
        } else {
            testUpdateConfigByName();
        }

        // ── 10. Update config by ARN ──────────────────────────────────────────
        section("10. Update Config by ARN");
        if (functionArnRaw == null) {
            skipAll("LAMBDA_TEST_ARN not set");
        } else {
            testUpdateConfigByArn();
        }

        // ── 11. LambdaArn value object tests ──────────────────────────────────
        section("11. LambdaArn Parsing & Validation");
        testArnParsing();

        // ── 12. Error cases ───────────────────────────────────────────────────
        section("12. Error Handling");
        testFunctionNotFound();
        testInvalidArnRejectedLocally();

        // ── Summary ───────────────────────────────────────────────────────────
        printSummary();
    }

    // ─────────────────────────────────────────────────────────────────────────
    // Individual test methods
    // ─────────────────────────────────────────────────────────────────────────

    private void testHealthCheck() {
        test("GET /api/v1/lambda/health returns 200", () -> {
            HttpClient http = HttpClient.newHttpClient();
            HttpRequest req = HttpRequest.newBuilder()
                    .uri(URI.create(baseUrl + "/api/v1/lambda/health"))
                    .GET()
                    .build();
            HttpResponse<String> resp = http.send(req, HttpResponse.BodyHandlers.ofString());
            assertEqual(200, resp.statusCode(), "status code");
            assertContains(resp.body(), "ok", "body contains 'ok'");
        });
    }

    private void testListFunctions() {
        test("listFunctions() returns a non-null list", () -> {
            List<FunctionMetadata> fns = client.listFunctions();
            assertNotNull(fns, "function list");
            log("  → found " + fns.size() + " function(s)");
            for (FunctionMetadata fn : fns) {
                assertNotNull(fn.getName(), "function name");
                log("    • " + fn.getName() + (fn.getArn() != null ? " [" + fn.getArn() + "]" : ""));
            }
        });
    }

    private void testGetFunctionByName() {
        test("getFunction(name) returns correct metadata", () -> {
            FunctionMetadata fn = client.getFunction(functionName);
            assertNotNull(fn, "function metadata");
            assertEqual(functionName, fn.getName(), "function name");
            assertNotNull(fn.getType(), "function type");
            log("  → type=" + fn.getType() + ", image=" + fn.getImage()
                    + ", arn=" + fn.getArn());
        });
    }

    private void testGetFunctionByArn() {
        test("getFunctionByArn(arn) returns correct metadata", () -> {
            LambdaArn arn = LambdaArn.parse(functionArnRaw);
            FunctionMetadata fn = client.getFunctionByArn(arn);
            assertNotNull(fn, "function metadata");
            assertNotNull(fn.getName(), "function name");
            log("  → name=" + fn.getName() + ", arn=" + fn.getArn());
        });

        test("getFunctionByArn(String) equals getFunctionByArn(LambdaArn)", () -> {
            FunctionMetadata byStr = client.getFunctionByArn(functionArnRaw);
            FunctionMetadata byObj = client.getFunctionByArn(LambdaArn.parse(functionArnRaw));
            assertNotNull(byStr, "by-string result");
            assertNotNull(byObj, "by-object result");
            assertEqual(byStr.getName(), byObj.getName(), "names match");
        });
    }

    private void testInvokeByName() {
        test("invoke(name, payload) returns a non-blank result", () -> {
            Map<String, Object> payload = Map.of("test", true, "source", "sdk-test");
            String result = client.invoke(functionName, payload);
            assertNotNull(result, "invoke result");
            log("  → result length: " + result.length() + " chars");
            log("  → preview: " + truncate(result, 200));
        });
    }

    private void testInvokeByArn() {
        test("invokeByArn(arn, payload) returns a non-blank result", () -> {
            Map<String, Object> payload = Map.of("test", true, "source", "sdk-test-arn");
            String result = client.invokeByArn(functionArnRaw, payload);
            assertNotNull(result, "invoke-by-arn result");
            log("  → result length: " + result.length() + " chars");
            log("  → preview: " + truncate(result, 200));
        });

        test("invokeByArn(LambdaArn, payload) overload works", () -> {
            LambdaArn arn = LambdaArn.parse(functionArnRaw);
            String result = client.invokeByArn(arn, Map.of());
            assertNotNull(result, "invoke-by-arn-object result");
        });
    }

    private void testGetMetricsByName() {
        test("getMetrics(name) returns non-null response", () -> {
            MetricsResponse metrics = client.getMetrics(functionName);
            assertNotNull(metrics, "metrics response");
            log("  → invocations=" + metrics.getInvocations()
                    + ", avgDuration=" + metrics.getDuration() + "ms"
                    + ", errors=" + metrics.getErrors());
        });
    }

    private void testGetMetricsByArn() {
        test("getMetricsByArn(arn) returns non-null response", () -> {
            MetricsResponse metrics = client.getMetricsByArn(functionArnRaw);
            assertNotNull(metrics, "metrics-by-arn response");
            log("  → invocations=" + metrics.getInvocations()
                    + ", errors=" + metrics.getErrors());
        });
    }

    private void testUpdateConfigByName() {
        test("updateConfig(name, config) does not throw", () -> {
            // Read current config first so we can restore
            FunctionMetadata fn = client.getFunction(functionName);
            int originalTimeout = fn.getTimeoutMs() > 0 ? fn.getTimeoutMs() / 1000 : 30;
            String originalDesc = fn.getDescription() != null ? fn.getDescription() : "";

            // Update with a test description
            client.updateConfig(functionName, Map.of(
                    "memory", 256,
                    "timeout", originalTimeout,
                    "description", originalDesc + " [sdk-test]"));

            // Verify
            FunctionMetadata updated = client.getFunction(functionName);
            assertContains(updated.getDescription(), "sdk-test", "description updated");
            log("  → description now: " + updated.getDescription());
        });
    }

    private void testUpdateConfigByArn() {
        test("updateConfigByArn(arn, config) does not throw", () -> {
            FunctionMetadata fn = client.getFunctionByArn(functionArnRaw);
            int originalTimeout = fn.getTimeoutMs() > 0 ? fn.getTimeoutMs() / 1000 : 30;

            client.updateConfigByArn(functionArnRaw, Map.of(
                    "memory", 256,
                    "timeout", originalTimeout,
                    "description", "Updated via ARN by sdk-test"));

            // Verify via name-based get
            FunctionMetadata updated = client.getFunction(fn.getName());
            assertContains(updated.getDescription(), "ARN", "description references ARN update");
            log("  → description now: " + updated.getDescription());
        });
    }

    private void testArnParsing() {
        test("LambdaArn.parse succeeds on valid ARN", () -> {
            String raw = "arn:serwin:lambda:us-east-1:123456789012:function:my-function";
            LambdaArn arn = LambdaArn.parse(raw);
            assertEqual("us-east-1", arn.getRegion(), "region");
            assertEqual("123456789012", arn.getAccountId(), "accountId");
            assertEqual("my-function", arn.getFunctionName(), "functionName");
            assertEqual(raw, arn.toString(), "round-trip");
        });

        test("LambdaArn.isValid returns true for valid ARN", () -> {
            assertTrue(LambdaArn.isValid("arn:serwin:lambda:eu-west-1:999:function:fn"), "valid ARN");
        });

        test("LambdaArn.isValid returns false for plain name", () -> {
            assertFalse(LambdaArn.isValid("my-function"), "plain name is not a valid ARN");
        });

        test("LambdaArn.isValid returns false for malformed ARN", () -> {
            assertFalse(LambdaArn.isValid("arn:aws:lambda:us-east-1"), "incomplete ARN");
        });

        test("LambdaArn.parse throws on null", () -> {
            assertThrows(IllegalArgumentException.class,
                    () -> LambdaArn.parse(null),
                    "parse(null) should throw");
        });

        test("LambdaArn.parse throws on missing :function: segment", () -> {
            assertThrows(IllegalArgumentException.class,
                    () -> LambdaArn.parse("arn:serwin:lambda:us-east-1:123"),
                    "missing :function: should throw");
        });
    }

    private void testFunctionNotFound() {
        test("getFunction(nonExistent) throws LambdaException with 404", () -> {
            assertThrows(LambdaException.class,
                    () -> client.getFunction("this-function-does-not-exist-xyz-9999"),
                    "should throw on not found");
            // If we get here we know LambdaException was thrown; check status code
            try {
                client.getFunction("this-function-does-not-exist-xyz-9999");
            } catch (LambdaException e) {
                assertEqual(404, e.getStatusCode(), "status code is 404");
                log("  → LambdaException(" + e.getStatusCode() + "): " + e.getMessage());
            }
        });
    }

    private void testInvalidArnRejectedLocally() {
        test("invokeByArn with bad ARN throws before network call", () -> {
            assertThrows(LambdaException.class,
                    () -> client.invokeByArn("not-an-arn", Map.of()),
                    "bad ARN string should throw LambdaException");
        });

        test("getFunctionByArn with bad ARN throws before network call", () -> {
            assertThrows(LambdaException.class,
                    () -> client.getFunctionByArn("arn:aws:lambda:us-east-1:123:function:fn"),
                    "AWS ARN (not serwin) should throw LambdaException");
        });
    }

    // ─────────────────────────────────────────────────────────────────────────
    // Test harness
    // ─────────────────────────────────────────────────────────────────────────

    @FunctionalInterface
    interface ThrowingRunnable {
        void run() throws Exception;
    }

    private void test(String name, ThrowingRunnable body) {
        System.out.print(CYAN + "  ▶ " + RESET + name + " ... ");
        try {
            body.run();
            System.out.println(GREEN + "PASS" + RESET);
            passed++;
        } catch (AssertionError | LambdaException e) {
            System.out.println(RED + "FAIL" + RESET);
            System.out.println(RED + "    ✗ " + e.getMessage() + RESET);
            failed++;
        } catch (Exception e) {
            System.out.println(RED + "ERROR" + RESET);
            System.out.println(RED + "    ✗ " + e.getClass().getSimpleName() + ": " + e.getMessage() + RESET);
            failed++;
        }
    }

    private void skipAll(String reason) {
        System.out.println(YELLOW + "  ↷ SKIPPED – " + reason + RESET);
        skipped++;
    }

    // ─────────────────────────────────────────────────────────────────────────
    // Assertion helpers
    // ─────────────────────────────────────────────────────────────────────────

    private void assertNotNull(Object value, String label) {
        if (value == null)
            throw new AssertionError(label + " must not be null");
    }

    private void assertEqual(Object expected, Object actual, String label) {
        if (!String.valueOf(expected).equals(String.valueOf(actual))) {
            throw new AssertionError(label + ": expected [" + expected + "] but got [" + actual + "]");
        }
    }

    private void assertContains(String text, String fragment, String label) {
        if (text == null || !text.contains(fragment)) {
            throw new AssertionError(label + ": expected [" + text + "] to contain [" + fragment + "]");
        }
    }

    private void assertTrue(boolean condition, String label) {
        if (!condition)
            throw new AssertionError(label + " should be true");
    }

    private void assertFalse(boolean condition, String label) {
        if (condition)
            throw new AssertionError(label + " should be false");
    }

    private <T extends Throwable> void assertThrows(
            Class<T> expectedType, ThrowingRunnable body, String label) {
        try {
            body.run();
            throw new AssertionError(
                    label + ": expected " + expectedType.getSimpleName() + " to be thrown, but nothing was thrown");
        } catch (Throwable t) {
            if (!expectedType.isInstance(t)) {
                throw new AssertionError(label + ": expected "
                        + expectedType.getSimpleName() + " but got "
                        + t.getClass().getSimpleName() + ": " + t.getMessage());
            }
        }
    }

    // ─────────────────────────────────────────────────────────────────────────
    // Utilities
    // ─────────────────────────────────────────────────────────────────────────

    private static String requireEnv(String name) {
        String val = System.getenv(name);
        if (val == null || val.isBlank()) {
            System.err.println(RED + "Required environment variable not set: " + name + RESET);
            System.exit(1);
        }
        return val;
    }

    private static void log(String msg) {
        System.out.println("  " + msg);
    }

    private static String truncate(String s, int max) {
        return s.length() <= max ? s : s.substring(0, max) + "...";
    }

    private static void section(String title) {
        System.out.println();
        System.out.println(BOLD + "── " + title + " " + RESET
                + "─".repeat(Math.max(0, 60 - title.length())));
    }

    private void printBanner() {
        System.out.println();
        System.out.println(BOLD + CYAN
                + "╔═══════════════════════════════════════════════════════╗\n"
                + "║     Serwin Lambda SDK  –  Integration Test Suite      ║\n"
                + "╚═══════════════════════════════════════════════════════╝"
                + RESET);
        System.out.println("  Base URL : " + System.getenv("LAMBDA_BASE_URL"));
        System.out.println("  Function : " + (functionName != null ? functionName : "(none – some tests skipped)"));
        System.out.println("  ARN      : " + (functionArnRaw != null ? functionArnRaw : "(none – ARN tests skipped)"));
    }

    private void printSummary() {
        System.out.println();
        System.out.println(BOLD + "──────────────────────────────────────────────────────────" + RESET);
        System.out.printf(BOLD + "  Results: %s%d passed%s, %s%d failed%s, %s%d skipped%s%n" + RESET,
                GREEN, passed, RESET,
                failed > 0 ? RED : "", failed, RESET,
                YELLOW, skipped, RESET);
        System.out.println(BOLD + "──────────────────────────────────────────────────────────" + RESET);

        if (failed > 0) {
            System.out.println(RED + "  ✗ Some tests failed. Review the output above." + RESET);
            System.exit(1);
        } else {
            System.out.println(GREEN + "  ✓ All tests passed!" + RESET);
        }
    }
}
