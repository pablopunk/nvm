import { Table, TableBody, TableHead, TableHeader, TableRow } from './ui/table';

interface Props {
  id: string;
  columns: string[];
}

export function ConsoleTable({ id, columns }: Props) {
  return (
    <Table id={id} className="table">
      <TableHeader>
        <TableRow>
          {columns.map((column, index) => <TableHead scope="col" key={`${column}-${index}`}>{column}</TableHead>)}
        </TableRow>
      </TableHeader>
      <TableBody />
    </Table>
  );
}
