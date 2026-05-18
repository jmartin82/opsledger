import { Toaster } from "@/components/ui/toaster";
import { Toaster as Sonner } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { AuthProvider } from "@/contexts/AuthContext";
import { LiveProvider } from "@/contexts/LiveContext";
import RequireAuth from "@/components/RequireAuth";
import Index from "./pages/Index";
import AddChange from "./pages/AddChange";
import Help from "./pages/Help";
import Admin from "./pages/Admin";
import Account from "./pages/Account";
import Login from "./pages/Login";
import CalendarPage from "./pages/Calendar";
import Connectors from "./pages/Connectors";
import NotFound from "./pages/NotFound";

const queryClient = new QueryClient();

const App = () => (
  <QueryClientProvider client={queryClient}>
    <TooltipProvider>
      <Toaster />
      <Sonner />
      <BrowserRouter>
        <AuthProvider>
          <LiveProvider>
            <Routes>
            {/* Public */}
            <Route path="/login" element={<Login />} />

            {/* Protected — any authenticated user */}
            <Route path="/" element={<RequireAuth><Index /></RequireAuth>} />
            <Route path="/calendar" element={<RequireAuth><CalendarPage /></RequireAuth>} />
            <Route path="/help" element={<RequireAuth><Help /></RequireAuth>} />
            <Route path="/account" element={<RequireAuth><Account /></RequireAuth>} />

            {/* Protected — editor or above */}
            <Route path="/add" element={<RequireAuth minRole="editor"><AddChange /></RequireAuth>} />

            {/* Protected — admin only */}
            <Route path="/admin" element={<RequireAuth minRole="admin"><Admin /></RequireAuth>} />
            <Route path="/connectors" element={<RequireAuth minRole="admin"><Connectors /></RequireAuth>} />

            {/* Catch-all */}
            <Route path="*" element={<NotFound />} />
            </Routes>
          </LiveProvider>
        </AuthProvider>
      </BrowserRouter>
    </TooltipProvider>
  </QueryClientProvider>
);

export default App;
