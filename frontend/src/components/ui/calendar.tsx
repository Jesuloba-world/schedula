import * as React from "react";
import { DayFlag, DayPicker, SelectionState, UI } from "react-day-picker";
import { ChevronLeft } from "lucide-react";

import { cn } from "../../lib/utils";

export type CalendarProps = React.ComponentProps<typeof DayPicker>;

export function Calendar({
	className,
	classNames,
	showOutsideDays = true,
	...props
}: CalendarProps) {
	const ghostButton =
		"inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 hover:bg-muted hover:text-foreground";
	const dayButton = cn(
		ghostButton,
		"h-9 w-9 rounded-md p-0 aria-selected:opacity-100",
	);
	return (
		<DayPicker
			showOutsideDays={showOutsideDays}
			className={cn("p-1", className)}
			classNames={{
				[UI.Months]: "flex flex-col gap-3",
				[UI.Month]: "space-y-3",
				[UI.MonthCaption]: "flex items-center justify-between px-1",
				[UI.CaptionLabel]: "text-sm font-semibold tracking-tight",
				[UI.Nav]: "flex items-center gap-1",
				[UI.PreviousMonthButton]: cn(ghostButton, "h-9 w-9 p-0"),
				[UI.NextMonthButton]: cn(ghostButton, "h-9 w-9 p-0"),
				[UI.MonthGrid]: "w-full border-collapse table-fixed",
				[UI.Weekdays]: "h-9",
				[UI.Weekday]:
					"w-9 p-0 text-center text-[0.72rem] font-medium text-muted-foreground",
				[UI.Weeks]: "",
				[UI.Week]: "",
				[UI.Day]: "h-9 w-9 p-0 text-center text-sm align-middle",
				[UI.DayButton]: dayButton,
				[SelectionState.selected]:
					"[&>button]:bg-primary [&>button]:text-primary-foreground [&>button]:hover:bg-primary [&>button]:hover:text-primary-foreground [&>button]:focus:bg-primary [&>button]:focus:text-primary-foreground",
				[SelectionState.range_start]:
					"[&>button]:bg-primary [&>button]:text-primary-foreground [&>button]:hover:bg-primary [&>button]:hover:text-primary-foreground [&>button]:focus:bg-primary [&>button]:focus:text-primary-foreground",
				[SelectionState.range_end]:
					"[&>button]:bg-primary [&>button]:text-primary-foreground [&>button]:hover:bg-primary [&>button]:hover:text-primary-foreground [&>button]:focus:bg-primary [&>button]:focus:text-primary-foreground",
				[SelectionState.range_middle]: "[&>button]:bg-muted",
				[DayFlag.today]:
					"[&>button]:ring-1 [&>button]:ring-ring [&>button]:ring-offset-2 [&>button]:ring-offset-background",
				[DayFlag.outside]:
					"[&>button]:text-muted-foreground/50 [&>button]:opacity-70",
				[DayFlag.disabled]:
					"[&>button]:text-muted-foreground/50 [&>button]:opacity-50",
				[DayFlag.hidden]: "invisible",
				...classNames,
			}}
			components={{
				Chevron: ({ className: iconClassName, orientation }) => {
					const rotation =
						orientation === "right"
							? "rotate-180"
							: orientation === "up"
								? "rotate-90"
								: orientation === "down"
									? "-rotate-90"
									: "";
					return (
						<ChevronLeft
							className={cn("h-4 w-4", rotation, iconClassName)}
						/>
					);
				},
			}}
			{...props}
		/>
	);
}
