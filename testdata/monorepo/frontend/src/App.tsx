import React from "react";
import { apiClient } from "../../backend/src/client";

export class AppShell {}

export function renderApp() {
  return apiClient;
}
