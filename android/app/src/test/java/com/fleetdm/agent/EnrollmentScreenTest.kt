package com.fleetdm.agent

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class EnrollmentScreenTest {
    @Test
    fun `accepts HTTPS and HTTP Fleet URLs`() {
        assertNull(validateEnrollmentInput("https://fleet.example.com", "secret"))
        assertNull(validateEnrollmentInput("http://192.168.1.20:8080", "secret"))
    }

    @Test
    fun `rejects missing or unsupported enrollment values`() {
        assertEquals("Fleet server URL is required", validateEnrollmentInput("", "secret"))
        assertEquals("Enrollment secret is required", validateEnrollmentInput("https://fleet.example.com", ""))
        assertEquals(
            "Fleet server URL must start with https:// or http://",
            validateEnrollmentInput("ftp://fleet.example.com", "secret"),
        )
    }
}
