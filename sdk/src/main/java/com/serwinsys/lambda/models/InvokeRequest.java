package com.serwinsys.lambda.models;

import java.util.Map;

public class InvokeRequest {
    private String name;
    private Map<String, Object> payload;

    public InvokeRequest() {}

    public InvokeRequest(String name, Map<String, Object> payload) {
        this.name = name;
        this.payload = payload;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

    public Map<String, Object> getPayload() {
        return payload;
    }

    public void setPayload(Map<String, Object> payload) {
        this.payload = payload;
    }
}
