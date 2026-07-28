import { createContext } from "react";

export const AppSessionContext = createContext({ auth: null, onLogout: null });
