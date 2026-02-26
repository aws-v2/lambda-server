package com.serwinsys.lambda.models;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import java.util.List;

@JsonIgnoreProperties(ignoreUnknown = true)
public class MetricsResponse {
    private int invocations;
    private double duration;
    private int errors;
    private List<TimelinePoint> timeline;

    public MetricsResponse() {}

    public int getInvocations() {
        return invocations;
    }

    public void setInvocations(int invocations) {
        this.invocations = invocations;
    }

    public double getDuration() {
        return duration;
    }

    public void setDuration(double duration) {
        this.duration = duration;
    }

    public int getErrors() {
        return errors;
    }

    public void setErrors(int errors) {
        this.errors = errors;
    }

    public List<TimelinePoint> getTimeline() {
        return timeline;
    }

    public void setTimeline(List<TimelinePoint> timeline) {
        this.timeline = timeline;
    }
}
