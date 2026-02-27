package com.serwinsys.lambda;

import com.serwinsys.lambda.models.InvokeResponse;
import com.serwinsys.lambda.models.MetricsResponse;
import com.serwinsys.lambda.config.*;
import java.util.Map;

public class Example {
    public static void main(String[] args) {
        // Initialize client with credentials
        SerwinCredentials credentials = new SerwinCredentials(
                System.getenv().getOrDefault("LAMBDA_ACCESS_KEY", "AKIAPQOQ22BFO8K2CT0Z"),
                System.getenv().getOrDefault("LAMBDA_SECRET_KEY", "pjEspdB9IeF6o98fXrMaQcPeF5K/a/JTSop64vjF"));

        SerwinLambdaClient client = new SerwinLambdaClient(credentials);

        try {
            // 1. List functions
            System.out.println("Listing functions...");
            client.listFunctions().forEach(fn -> System.out.println(" - " + fn.getName() + " (" + fn.getImage() + ")"));

            // 2. Invoke a function
            System.out.println("\nInvoking hello-gow...");
            InvokeResponse result = client.invoke("hello-gow", Map.of("name", "SDK User"));
            System.out.println("Result: " + result);

            // 3. Get Metrics
            System.out.println("\nMetrics for hello-gow:");
            MetricsResponse metrics = client.getMetrics("hello-gow");
            System.out.println(" Invocations: " + metrics.getInvocations());
            System.out.println(" Avg Duration: " + metrics.getDuration() + "ms");

            // 4. Test ARN invocation
            client.listFunctions().stream()
                    .filter(fn -> "another-test".equals(fn.getName()))
                    .findFirst()
                    .ifPresent(fn -> {
                        System.out.println("\nFound function: " + fn.getName() + " with ARN: " + fn.getArn());
                        testInvokeByArn(client, fn.getArn());
                    });

        } catch (Exception e) {
            System.err.println("Error: " + e.getMessage());
            e.printStackTrace();
        }
    }

    private static void testInvokeByArn(SerwinLambdaClient client, String arn) {
        System.out.println("\nInvoking by ARN: " + arn);
        InvokeResponse result = client.invokeByArn(arn, Map.of("name", "ARN User"));
        System.out.println("ARN Invocation Result: " + result);
    }
}
