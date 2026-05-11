import {
  Play, BookOpen, GraduationCap, Home, Inbox, Bell,
  FileText, PenTool, HelpCircle, Calendar, CheckSquare,
  Users, MessageSquare, FolderOpen, Award, BarChart2,
  Clock, Star, Heart, Lightbulb, Pencil, Search,
  Globe, Music, Palette, Camera, Mic, Video,
  Rocket, Target
} from 'lucide-react';

const iconMap = {
  Play, BookOpen, GraduationCap, Home, Inbox, Bell,
  FileText, PenTool, HelpCircle, Calendar, CheckSquare,
  Users, MessageSquare, FolderOpen, Award, BarChart2,
  Clock, Star, Heart, Lightbulb, Pencil, Search,
  Globe, Music, Palette, Camera, Mic, Video,
  Rocket, Target,
};

export const resolveIcon = (name) => {
  if (!name) return null;
  return iconMap[name] || iconMap.BookOpen;
};

export const availableIcons = Object.keys(iconMap);
